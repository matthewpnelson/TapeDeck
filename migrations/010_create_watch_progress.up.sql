-- Track watch progress per Plex user for TV shows
-- Used to determine next episode for managed users (whose tokens can't access Plex API directly)
CREATE TABLE IF NOT EXISTS watch_progress (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plex_account_id TEXT NOT NULL,       -- Plex user ID from webhook (e.g., "269359635")
    plex_account_name TEXT NOT NULL,     -- Plex username for display
    plex_server_id TEXT NOT NULL,        -- Server UUID
    show_rating_key TEXT NOT NULL,       -- TV show's ratingKey (for grouping episodes)
    episode_rating_key TEXT NOT NULL,    -- Episode's ratingKey
    season_number INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    episode_title TEXT,
    completed_at DATETIME NOT NULL,      -- When the episode was marked complete (scrobble time)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Each user can only complete each episode once
    UNIQUE(plex_account_id, episode_rating_key)
);

-- Index for looking up user's watch progress
CREATE INDEX idx_watch_progress_account ON watch_progress(plex_account_id);

-- Index for finding last watched episode of a show for a user
CREATE INDEX idx_watch_progress_show ON watch_progress(plex_account_id, show_rating_key, completed_at);
