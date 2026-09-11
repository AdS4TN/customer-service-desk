package enums

type TranslationLanguage string

const (
	TranslationLanguageAuto               TranslationLanguage = "auto"
	TranslationLanguageChinese            TranslationLanguage = "zh-CN"
	TranslationLanguageTraditionalChinese TranslationLanguage = "zh-TW"
	TranslationLanguageEnglish            TranslationLanguage = "en"
	TranslationLanguageSpanish            TranslationLanguage = "es"
	TranslationLanguagePortuguese         TranslationLanguage = "pt"
	TranslationLanguageFrench             TranslationLanguage = "fr"
	TranslationLanguageGerman             TranslationLanguage = "de"
	TranslationLanguageItalian            TranslationLanguage = "it"
	TranslationLanguageRussian            TranslationLanguage = "ru"
	TranslationLanguageArabic             TranslationLanguage = "ar"
	TranslationLanguageJapanese           TranslationLanguage = "ja"
	TranslationLanguageKorean             TranslationLanguage = "ko"
	TranslationLanguageVietnamese         TranslationLanguage = "vi"
	TranslationLanguageThai               TranslationLanguage = "th"
	TranslationLanguageIndonesian         TranslationLanguage = "id"
	TranslationLanguageTurkish            TranslationLanguage = "tr"
	TranslationLanguageHindi              TranslationLanguage = "hi"
)

var translationLanguageLabelMap = map[TranslationLanguage]string{
	TranslationLanguageAuto: "自动识别", TranslationLanguageChinese: "简体中文", TranslationLanguageTraditionalChinese: "繁体中文",
	TranslationLanguageEnglish: "English", TranslationLanguageSpanish: "Español", TranslationLanguagePortuguese: "Português",
	TranslationLanguageFrench: "Français", TranslationLanguageGerman: "Deutsch", TranslationLanguageItalian: "Italiano",
	TranslationLanguageRussian: "Русский", TranslationLanguageArabic: "العربية", TranslationLanguageJapanese: "日本語",
	TranslationLanguageKorean: "한국어", TranslationLanguageVietnamese: "Tiếng Việt", TranslationLanguageThai: "ไทย",
	TranslationLanguageIndonesian: "Bahasa Indonesia", TranslationLanguageTurkish: "Türkçe", TranslationLanguageHindi: "हिन्दी",
}

func GetTranslationLanguageLabel(language TranslationLanguage) string {
	return translationLanguageLabelMap[language]
}
