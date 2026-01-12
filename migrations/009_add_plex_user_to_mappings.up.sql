-- Add Plex Home user fields to card_mappings
-- Allows each card to specify which Plex user's watch history to use
ALTER TABLE card_mappings ADD COLUMN plex_user_uuid TEXT;
ALTER TABLE card_mappings ADD COLUMN plex_user_name TEXT;
