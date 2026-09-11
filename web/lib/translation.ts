import { TranslationLanguage, TranslationLanguageLabels } from "@/lib/generated/enums";

export function translationLanguageOptions(autoLabel: string, includeAuto = false) {
  return Object.values(TranslationLanguage).filter((value) => includeAuto || value !== TranslationLanguage.Auto)
    .map((value) => ({ value, label: value === TranslationLanguage.Auto ? autoLabel : TranslationLanguageLabels[value] }));
}

export function translatedTextHTML(text: string) {
  const escape = (line: string) => line.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  return text.split(/\r?\n/).map((line) => `<p>${escape(line) || "<br>"}</p>`).join("");
}

export type TranslationContext = { id: number; lastMessageId: number; currentAssigneeId?: number; status: number; customerId?: number; channelId?: number; aiAgentId?: number };

export function isTranslationContextCurrent(original: TranslationContext, current: TranslationContext | null | undefined, selectedId: number | null) {
  return !!current && selectedId === original.id && current.id === original.id && current.lastMessageId === original.lastMessageId &&
    current.currentAssigneeId === original.currentAssigneeId && current.status === original.status && current.customerId === original.customerId &&
    current.channelId === original.channelId && current.aiAgentId === original.aiAgentId;
}

// Read the editor's HTML structurally, but never feed attachments or markup to translation.
export function translationDraftText(html: string) {
  const doc = new DOMParser().parseFromString(html, "text/html");
  if (doc.querySelector("img,video,audio,iframe,object,embed,svg,canvas")) return null;
  for (const node of doc.querySelectorAll("script,style")) node.remove();
  for (const node of doc.querySelectorAll("a[href]")) {
    const href = node.getAttribute("href") ?? "";
    if (/^(https?:\/\/|mailto:)/i.test(href) && node.textContent !== href) node.append(doc.createTextNode(` (${href})`));
  }
  for (const node of doc.querySelectorAll("br")) node.replaceWith(doc.createTextNode("\n"));
  for (const node of doc.querySelectorAll("p,div,li,h1,h2,h3,blockquote,pre")) node.append(doc.createTextNode("\n"));
  return (doc.body.textContent ?? "").trim();
}
