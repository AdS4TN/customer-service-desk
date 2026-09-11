package services

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Only synthetic translation data goes to the model. No channel or live message writes.
func TestTranslationLive(t *testing.T) {
	path := os.Getenv("AGENT_DESK_TRANSLATION_LIVE_DB")
	if path == "" {
		t.Skip("opt in with AGENT_DESK_TRANSLATION_LIVE_DB and AGENT_DESK_TRANSLATION_LIVE_CONFIG")
	}
	id, err := strconv.ParseInt(os.Getenv("AGENT_DESK_TRANSLATION_LIVE_CONFIG"), 10, 64)
	if err != nil || id <= 0 {
		t.Fatal("valid config ID required")
	}
	db, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatal("cannot read model configuration")
	}
	conn, _ := db.DB()
	defer conn.Close()
	var cfg models.AIConfig
	if db.First(&cfg, id).Error != nil {
		t.Fatal("model configuration missing")
	}
	s := &conversationTranslationService{complete: ai.LLM.ChatWithConfig}
	for _, tc := range []struct {
		name, text, sample string
		target             enums.TranslationLanguage
		expected           enums.TranslationLanguage
		must               []string
	}{
		{"en_zh", "We need 250 units of AX-204. Is the price USD 12.50 per unit? We have NOT confirmed the order.", "", enums.TranslationLanguageChinese, enums.TranslationLanguageChinese, []string{"250", "AX-204", "12.50"}},
		{"zh_en", "AX-204 的价格尚未确认，不能承诺 7 天内交付。数量为 250 件。", "", enums.TranslationLanguageEnglish, enums.TranslationLanguageEnglish, []string{"AX-204", "7", "250"}},
		{"auto_spanish", "我们已收到您对 BX-12 的询价，请问需要多少件？", "Hola, necesito una cotización para BX-12.", enums.TranslationLanguageAuto, enums.TranslationLanguageSpanish, []string{"BX-12"}},
		{"arabic_zh", "أحتاج إلى 100 قطعة من AX-204، ولا أوافق على سعر 50 دولارًا.", "", enums.TranslationLanguageChinese, enums.TranslationLanguageChinese, []string{"100", "AX-204", "50"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := time.Now()
			r, err := s.translate(context.Background(), cfg, tc.text, tc.target, tc.sample)
			if err != nil {
				t.Fatal("live translation failed; provider details omitted")
			}
			if r.TargetLanguage != tc.expected {
				t.Fatal("incorrect target language")
			}
			for _, term := range tc.must {
				if !strings.Contains(r.Content, term) {
					t.Errorf("missing synthetic fact %s", term)
				}
			}
			t.Logf("%s | %s | %s", tc.name, time.Since(started).Round(time.Millisecond), r.Content)
		})
	}
}
