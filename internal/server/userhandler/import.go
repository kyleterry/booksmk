package userhandler

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"go.e64ec.com/booksmk/internal/auth"
	"go.e64ec.com/booksmk/internal/importer"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	userpages "go.e64ec.com/booksmk/internal/ui/users"
)

const maxImportSize = 10 << 20 // 10 MB

func (h *Handler) handleImport(w http.ResponseWriter, r *http.Request) {
	authedUser, _ := auth.UserFromContext(r.Context())

	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if id != authedUser.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	renderSettings := func(importErrMsg string) {
		user, err := h.store.GetUser(r.Context(), id)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		keys, err := h.store.ListAPIKeys(r.Context(), id)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}
		baseURL := scheme + "://" + r.Host

		h.render(w, r, ui.Base("settings", h.navUser(r), userpages.UserDetailPage(user, keys, baseURL, importErrMsg)))
	}

	if err := r.ParseMultipartForm(maxImportSize); err != nil {
		renderSettings("could not read upload")
		return
	}

	file, header, err := r.FormFile("bookmarks_file")
	if err != nil {
		renderSettings("no file selected")
		return
	}
	defer func() { _ = file.Close() }()

	ext := filepath.Ext(header.Filename)

	var bookmarks []importer.Bookmark

	switch ext {
	case ".html", ".htm":
		bookmarks, err = importer.ParseNetscapeHTML(file)
	case ".csv":
		bookmarks, err = importer.ParsePinboardCSV(file)
	default:
		renderSettings("unsupported file type; upload an .html or .csv file")
		return
	}

	if errors.Is(err, importer.ErrUnrecognisedFormat) {
		renderSettings("unrecognised CSV format; expected a Pinboard or Delicious export")
		return
	}
	if err != nil {
		h.logger.Error("failed to parse bookmark file", "filename", header.Filename, "error", err)
		renderSettings("failed to parse file")
		return
	}

	var (
		imported int
		failures []userpages.ImportFailure
	)

	for _, bm := range bookmarks {
		if bm.URL == "" {
			continue
		}

		tags := make([]string, 0, len(bm.Tags))
		for _, t := range bm.Tags {
			if s := store.Slug(t); s != "" {
				tags = append(tags, s)
			}
		}

		isBlocked, err := h.urlStore.IsBlocked(r.Context(), bm.URL)
		if err != nil {
			h.logger.Error("import: failed to check blocklist", "url", bm.URL, "error", err)
		}

		if isBlocked && !authedUser.IsAdmin {
			failures = append(failures, userpages.ImportFailure{URL: bm.URL, Reason: "this URL or domain is blocked"})
			continue
		}

		_, createErr := h.urlStore.CreateURL(r.Context(), authedUser.ID, bm.URL, bm.Title, bm.Description, tags, isBlocked)
		if createErr != nil {

			h.logger.Warn("import: failed to create url", "url", bm.URL, "error", createErr)
			failures = append(failures, userpages.ImportFailure{URL: bm.URL, Reason: fmt.Sprintf("could not save: %s", createErr)})
			continue
		}

		imported++
	}

	h.render(w, r, ui.Base("import complete", h.navUser(r), userpages.ImportResultPage(id.String(), imported, failures)))
}
