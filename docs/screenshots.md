# Screenshots

## Bookmarks

### URL List

The main bookmarks view. Shows all saved URLs with titles, tags, and the date
added.

![URL list view](images/url-list-view-01.png)

### URL Detail

Detail view for a single bookmark showing the URL, title, tags, and date added.
Also surfaces related discussions found on Hacker News.

![URL detail view](images/url-detail-view-01.png)

### Add URL

Form for saving a new URL. The title field is optional; booksmk will fetch the
page title automatically if left blank. Tags are entered as a comma-separated
list.

![Add URL form](images/url-create-view-01.png)

### URL Detail with Feed Subscribe

When a saved URL has an associated RSS/Atom feed, a subscribe button appears on
the detail page. It tries its best to look for feed URLs in the page, but
sometimes it doesn't find them. I'm working on this... there's a lot of edge cases.

![URL detail with subscribe button](images/url-detail-with-subscribe-button-01.png)

## Categories

### Category List

Categories group bookmarks by tag and domain filters. Each category acts as a
saved filter that aggregates matching URLs into a single view.

![Category list view](images/category-list-view-01.png)

### Edit Category

Category editor where you define the name and member filters. Filters are
comma-separated expressions in the form `tag:name` or `domain:hostname`.

![Category edit view](images/category-edit-view-01.png)

## Feeds

### Feed List

The feeds page lists all subscribed RSS/Atom feeds. Each entry shows the feed
title and source. Items can be marked read or saved as bookmarks.

![Feed list view 1](images/feed-main-view-01.png)
![Feed list view 2](images/feed-main-view-02.png)

### Feed Detail

Detail view for a single feed showing individual items with titles,
descriptions, and links.

![Feed detail view](images/feed-detail-view-01.png)

## Settings

### User Settings (Light Mode)

User settings page showing email management, theme selection (light/dark), font
size, and API key management. API keys are used for programmatic access using
the `Autorization` header.

![Settings view in light mode](images/settings-view-light-mode-01.png)
