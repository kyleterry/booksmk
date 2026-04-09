package importer_test

import (
	"strings"
	"testing"

	"go.e64ec.com/booksmk/internal/importer"
)

func TestParseNetscapeHTML(t *testing.T) {
	t.Run("firefox export with tags and description", func(t *testing.T) {
		input := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><A HREF="https://example.com" ADD_DATE="1234567890" TAGS="go,tools">Example Site</A>
    <DD>A useful website
    <DT><A HREF="https://other.example.com" ADD_DATE="1234567891">No Tags No Desc</A>
</DL><p>`

		got, err := importer.ParseNetscapeHTML(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 bookmarks, got %d", len(got))
		}

		cases := []struct {
			url, title, desc string
			tags             []string
		}{
			{"https://example.com", "Example Site", "A useful website", []string{"go", "tools"}},
			{"https://other.example.com", "No Tags No Desc", "", nil},
		}
		for i, tc := range cases {
			bm := got[i]
			if bm.URL != tc.url {
				t.Errorf("[%d] URL: want %q, got %q", i, tc.url, bm.URL)
			}
			if bm.Title != tc.title {
				t.Errorf("[%d] Title: want %q, got %q", i, tc.title, bm.Title)
			}
			if bm.Description != tc.desc {
				t.Errorf("[%d] Description: want %q, got %q", i, tc.desc, bm.Description)
			}
			if len(bm.Tags) != len(tc.tags) {
				t.Errorf("[%d] Tags len: want %d, got %d (%v)", i, len(tc.tags), len(bm.Tags), bm.Tags)
				continue
			}
			for j, tag := range tc.tags {
				if bm.Tags[j] != tag {
					t.Errorf("[%d] Tags[%d]: want %q, got %q", i, j, tag, bm.Tags[j])
				}
			}
		}
	})

	t.Run("empty file", func(t *testing.T) {
		got, err := importer.ParseNetscapeHTML(strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("want 0 bookmarks, got %d", len(got))
		}
	})

	t.Run("no description line", func(t *testing.T) {
		input := `<DT><A HREF="https://nodesc.example.com" TAGS="test">Title Only</A>`
		got, err := importer.ParseNetscapeHTML(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 bookmark, got %d", len(got))
		}
		if got[0].Description != "" {
			t.Errorf("Description: want empty, got %q", got[0].Description)
		}
	})

	t.Run("chrome export without TAGS attribute", func(t *testing.T) {
		input := `<DT><A HREF="https://chrome.example.com" ADD_DATE="1234567890">Chrome Bookmark</A>`
		got, err := importer.ParseNetscapeHTML(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 bookmark, got %d", len(got))
		}
		if len(got[0].Tags) != 0 {
			t.Errorf("Tags: want none, got %v", got[0].Tags)
		}
	})
}

func TestParsePinboardCSV(t *testing.T) {
	t.Run("standard pinboard export", func(t *testing.T) {
		input := `"href","description","extended","meta","hash","time","shared","toread","tags"
"https://pinboard.example.com","Pinboard Title","Some notes","abc","def","2023-01-15T10:30:00Z","yes","no","go tools reference"
"https://other.pinboard.example.com","Other Title","","ghi","jkl","2023-01-16T11:00:00Z","yes","no",""`

		got, err := importer.ParsePinboardCSV(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 bookmarks, got %d", len(got))
		}

		bm := got[0]
		if bm.URL != "https://pinboard.example.com" {
			t.Errorf("URL: want https://pinboard.example.com, got %q", bm.URL)
		}
		if bm.Title != "Pinboard Title" {
			t.Errorf("Title: want %q, got %q", "Pinboard Title", bm.Title)
		}
		if bm.Description != "Some notes" {
			t.Errorf("Description: want %q, got %q", "Some notes", bm.Description)
		}
		wantTags := []string{"go", "tools", "reference"}
		if len(bm.Tags) != len(wantTags) {
			t.Fatalf("Tags len: want %d, got %d (%v)", len(wantTags), len(bm.Tags), bm.Tags)
		}
		for i, tag := range wantTags {
			if bm.Tags[i] != tag {
				t.Errorf("Tags[%d]: want %q, got %q", i, tag, bm.Tags[i])
			}
		}

		// Second bookmark has no tags.
		if len(got[1].Tags) != 0 {
			t.Errorf("second bookmark Tags: want none, got %v", got[1].Tags)
		}
	})

	t.Run("unrecognised header returns error", func(t *testing.T) {
		input := `"url","name","note"
"https://example.com","test",""`

		_, err := importer.ParsePinboardCSV(strings.NewReader(input))
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("empty file returns nil", func(t *testing.T) {
		got, err := importer.ParsePinboardCSV(strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("want 0 bookmarks, got %d", len(got))
		}
	})
}
