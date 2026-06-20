package memory

import "github.com/danceable/provider/examples/blog/infrastructure/i18n"

// translations holds every UI string keyed by language and then by message key.
// It is hard-coded so the example needs no translation files or external
// service. Every language defines the same set of keys (enforced by tests).
var translations = map[i18n.Language]map[string]string{
	i18n.English: {
		"app.name":                 "Blog",
		"nav.home":                 "Home",
		"nav.dashboard":            "Dashboard",
		"nav.language":             "Language",
		"home.heading":             "Latest articles",
		"home.empty":               "No articles yet.",
		"home.empty_cta":           "Write one in the dashboard",
		"pagination.previous":      "Previous",
		"pagination.next":          "Next",
		"pagination.page":          "Page",
		"pagination.of":            "of",
		"article.back":             "Back to all articles",
		"dashboard.new":            "New article",
		"dashboard.empty":          "No articles yet.",
		"dashboard.confirm_delete": "Delete this article?",
		"field.title":              "Title",
		"field.created":            "Created",
		"field.actions":            "Actions",
		"field.body":               "Body",
		"action.edit":              "Edit",
		"action.delete":            "Delete",
		"action.save":              "Save",
		"action.cancel":            "Cancel",
		"form.new":                 "New article",
		"form.edit":                "Edit article",
		"error.back_home":          "Back home",
		"error.empty_title":        "Title must not be empty",
		"error.empty_body":         "Body must not be empty",
		"error.invalid":            "Invalid input",
	},
	i18n.German: {
		"app.name":                 "Blog",
		"nav.home":                 "Startseite",
		"nav.dashboard":            "Verwaltung",
		"nav.language":             "Sprache",
		"home.heading":             "Neueste Artikel",
		"home.empty":               "Noch keine Artikel.",
		"home.empty_cta":           "Schreibe einen in der Verwaltung",
		"pagination.previous":      "Zurück",
		"pagination.next":          "Weiter",
		"pagination.page":          "Seite",
		"pagination.of":            "von",
		"article.back":             "Zurück zu allen Artikeln",
		"dashboard.new":            "Neuer Artikel",
		"dashboard.empty":          "Noch keine Artikel.",
		"dashboard.confirm_delete": "Diesen Artikel löschen?",
		"field.title":              "Titel",
		"field.created":            "Erstellt",
		"field.actions":            "Aktionen",
		"field.body":               "Inhalt",
		"action.edit":              "Bearbeiten",
		"action.delete":            "Löschen",
		"action.save":              "Speichern",
		"action.cancel":            "Abbrechen",
		"form.new":                 "Neuer Artikel",
		"form.edit":                "Artikel bearbeiten",
		"error.back_home":          "Zur Startseite",
		"error.empty_title":        "Der Titel darf nicht leer sein",
		"error.empty_body":         "Der Inhalt darf nicht leer sein",
		"error.invalid":            "Ungültige Eingabe",
	},
	i18n.Persian: {
		"app.name":                 "وبلاگ",
		"nav.home":                 "خانه",
		"nav.dashboard":            "داشبورد",
		"nav.language":             "زبان",
		"home.heading":             "تازه‌ترین نوشته‌ها",
		"home.empty":               "هنوز نوشته‌ای نیست.",
		"home.empty_cta":           "در داشبورد یکی بنویسید",
		"pagination.previous":      "قبلی",
		"pagination.next":          "بعدی",
		"pagination.page":          "صفحه",
		"pagination.of":            "از",
		"article.back":             "بازگشت به همه نوشته‌ها",
		"dashboard.new":            "نوشتهٔ جدید",
		"dashboard.empty":          "هنوز نوشته‌ای نیست.",
		"dashboard.confirm_delete": "این نوشته حذف شود؟",
		"field.title":              "عنوان",
		"field.created":            "تاریخ ایجاد",
		"field.actions":            "عملیات",
		"field.body":               "متن",
		"action.edit":              "ویرایش",
		"action.delete":            "حذف",
		"action.save":              "ذخیره",
		"action.cancel":            "انصراف",
		"form.new":                 "نوشتهٔ جدید",
		"form.edit":                "ویرایش نوشته",
		"error.back_home":          "بازگشت به خانه",
		"error.empty_title":        "عنوان نباید خالی باشد",
		"error.empty_body":         "متن نباید خالی باشد",
		"error.invalid":            "ورودی نامعتبر است",
	},
	i18n.Chinese: {
		"app.name":                 "博客",
		"nav.home":                 "首页",
		"nav.dashboard":            "仪表板",
		"nav.language":             "语言",
		"home.heading":             "最新文章",
		"home.empty":               "还没有文章。",
		"home.empty_cta":           "去仪表板写一篇",
		"pagination.previous":      "上一页",
		"pagination.next":          "下一页",
		"pagination.page":          "第",
		"pagination.of":            "共",
		"article.back":             "返回所有文章",
		"dashboard.new":            "新建文章",
		"dashboard.empty":          "还没有文章。",
		"dashboard.confirm_delete": "删除这篇文章？",
		"field.title":              "标题",
		"field.created":            "创建时间",
		"field.actions":            "操作",
		"field.body":               "正文",
		"action.edit":              "编辑",
		"action.delete":            "删除",
		"action.save":              "保存",
		"action.cancel":            "取消",
		"form.new":                 "新建文章",
		"form.edit":                "编辑文章",
		"error.back_home":          "返回首页",
		"error.empty_title":        "标题不能为空",
		"error.empty_body":         "正文不能为空",
		"error.invalid":            "输入无效",
	},
}

// TranslationRepository is an in-memory i18n.Repository backed by the hard-coded
// translations table above.
type TranslationRepository struct{}

// compile-time assertion that the type satisfies the i18n.Repository port.
var _ i18n.Repository = (*TranslationRepository)(nil)

// NewTranslationRepository returns the in-memory translation repository.
func NewTranslationRepository() *TranslationRepository { return &TranslationRepository{} }

// Translate returns the value for key in lang and whether it exists.
func (r *TranslationRepository) Translate(lang i18n.Language, key string) (string, bool) {
	v, ok := translations[lang][key]
	return v, ok
}
