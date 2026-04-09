package userhandler

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"go.e64ec.com/booksmk/internal/reqctx"
	"go.e64ec.com/booksmk/internal/ui"
	userpages "go.e64ec.com/booksmk/internal/ui/users"
	"go.e64ec.com/booksmk/internal/importer"
)

// toSlug lowercases s and converts it to a URL-safe slug.
func toSlug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

const maxImportSize = 10 << 20 // 10 MB

func (h *Handler) handleImport(w http.ResponseWriter, r *http.Request) {
	authedUser, ok := reqctx.User(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

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
			if s := toSlug(t); s != "" {
				tags = append(tags, s)
			}
		}

		_, createErr := h.urlStore.CreateURL(r.Context(), authedUser.ID, bm.URL, bm.Title, bm.Description, tags)
		if createErr != nil {
			h.logger.Warn("import: failed to create url", "url", bm.URL, "error", createErr)
			failures = append(failures, userpages.ImportFailure{URL: bm.URL, Reason: fmt.Sprintf("could not save: %s", createErr)})
			continue
		}

		imported++
	}

	h.render(w, r, ui.Base("import complete", h.navUser(r), userpages.ImportResultPage(id.String(), imported, failures)))
}
