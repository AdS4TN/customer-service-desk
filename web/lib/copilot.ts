// TipTap JSON treats model output as literal text, never HTML or executable links.
export function suggestionParagraphs(text: string) {
  return text.split(/\r?\n/).map((line) => ({ type: "paragraph", ...(line ? { content: [{ type: "text", text: line }] } : {}) }));
}

export function isCurrentSuggestion(suggestion: { conversationId: number; lastMessageId: number }, conversation: { id: number; lastMessageId: number } | null) {
  return !!conversation && suggestion.conversationId === conversation.id && suggestion.lastMessageId === conversation.lastMessageId;
}
