// Temporary script to list Plex Home users and their IDs
// Usage: go run ./cmd/backfill
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Chuntttttt/tapedeck/internal/config"
	"github.com/Chuntttttt/tapedeck/internal/db"
	"github.com/Chuntttttt/tapedeck/internal/plex"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabasePath(), cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Get user 1 from database (assumes single-user setup)
	user, err := database.GetUserByID(ctx, 1)
	if err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}
	fmt.Printf("TapeDeck user: %s\n\n", user.PlexUsername)

	// Initialize Plex auth client
	plexAuth := plex.NewAuthClient("https://plex.tv", "tapedeck-client-id", "TapeDeck", cfg.DevMode)

	// Fetch Plex Home users
	homeUsers, err := plexAuth.GetHomeUsers(ctx, user.PlexAuthToken)
	if err != nil {
		log.Fatalf("Failed to fetch Plex Home users: %v", err)
	}

	fmt.Println("Plex Home Users:")
	fmt.Println("================")
	for _, u := range homeUsers {
		managed := ""
		if u.Restricted {
			managed = " (Managed)"
		}
		fmt.Printf("  ID: %-10s  Name: %s%s\n", u.ID, u.Title, managed)
	}
	fmt.Println()
	fmt.Println("To backfill all mappings, run:")
	fmt.Println("  sqlite3 tapedeck.db \"UPDATE card_mappings SET plex_user_uuid='<ID>', plex_user_name='<Name>' WHERE plex_user_uuid IS NULL OR plex_user_uuid='';\"")
}
