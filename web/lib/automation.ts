import type { AutomationDraft } from "@/lib/api/automation";

export function automationTemplate(kind: string, title: string, tag: string): AutomationDraft {
  const draft: AutomationDraft = { id: 0, revision: 0, name: title, priority: 100, definition: { trigger: "message_received", match: "any", oncePerConversation: false, conditions: [], actions: [{ type: "extract_lead", value: "", ownerId: 0 }] } };
  if (kind === "ticket") {
    draft.definition.oncePerConversation = true;
    draft.definition.conditions = [{ field: "text", value: "售后" }, { field: "text", value: "refund" }, { field: "text", value: "broken" }];
    draft.definition.actions = [{ type: "create_ticket", value: title, ownerId: 0 }];
  } else if (kind === "lead") {
    draft.definition.trigger = "lead_created";
    draft.definition.match = "all";
    draft.definition.actions = [{ type: "tag_lead", value: tag, ownerId: 0 }];
  } else {
    draft.definition.conditions = [{ field: "text", value: "报价" }, { field: "text", value: "quote" }, { field: "text", value: "price" }];
  }
  return draft;
}

export function automationResultLink(result: string): string | null {
  const [kind, id] = result.split(":");
  if (!/^[1-9]\d*$/.test(id ?? "")) return null;
  if (kind === "ticket") return `/dashboard/tickets?ticketId=${id}`;
  if (kind === "assign_lead" || kind === "tag_lead") return `/dashboard/sales-leads?leadId=${id}`;
  return null;
}
