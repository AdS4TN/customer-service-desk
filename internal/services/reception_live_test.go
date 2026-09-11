package services

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Opt-in provider acceptance check. Only synthetic messages leave the machine;
// the configured database is opened read-only and no channel dispatch occurs.
func TestReceptionLiveExtraction(t *testing.T) {
	path := os.Getenv("AGENT_DESK_RECEPTION_LIVE_DB")
	if path == "" {
		t.Skip("set AGENT_DESK_RECEPTION_LIVE_DB and AGENT_DESK_RECEPTION_LIVE_CONFIG to opt in")
	}
	id, err := strconv.ParseInt(os.Getenv("AGENT_DESK_RECEPTION_LIVE_CONFIG"), 10, 64)
	if err != nil || id <= 0 {
		t.Fatal("valid config ID required")
	}
	readDB, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatal("cannot open configured database")
	}
	conn, err := readDB.DB()
	if err != nil {
		t.Fatal("cannot open database connection")
	}
	defer conn.Close()
	var config models.AIConfig
	if err := readDB.First(&config, id).Error; err != nil {
		t.Fatal("model config unavailable")
	}
	db, c, _ := setupMemoryTest(t)
	config.ID = 0
	config.MaxRetryCount = 0
	config.MaxOutputTokens = 2500
	if err := db.Create(&config).Error; err != nil {
		t.Fatal("cannot create isolated model config")
	}
	p := testReceptionPolicy()
	raw, _ := json.Marshal(p)
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Updates(map[string]any{"ai_config_id": config.ID, "reception_policy": string(raw)}).Error; err != nil {
		t.Fatal(err)
	}
	s := &conversationMemoryService{complete: func(ctx context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		return ai.LLM.ChatWithConfig(ctx, cfg, system, input)
	}}
	start := time.Now()
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal("live extraction failed; provider details intentionally omitted")
	}
	v, err := s.View(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range v.Entries {
		if entry.Entry.FieldKey == "region" {
			t.Fatal("invented unknown region")
		}
		if entry.Entry.FieldKey == "quantity" && len(entry.Sources) == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("provider did not extract the stated quantity with evidence")
	}
	t.Logf("Live extraction: configured quantity with source, unknown region omitted; elapsed %s", time.Since(start).Round(time.Millisecond))
}
