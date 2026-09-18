import { request } from "@/lib/api/client";

export type AutomationCondition = { field: string; value: string };
export type AutomationAction = { type: string; value: string; ownerId: number };
export type AutomationDefinition = {
  trigger: string;
  match: string;
  oncePerConversation: boolean;
  conditions: AutomationCondition[];
  actions: AutomationAction[];
};
export type AutomationRule = { id: number; revision: number; name: string; priority: number; enabled: boolean; definition: AutomationDefinition; updatedAt: string };
export type AutomationDraft = Pick<AutomationRule, "id" | "revision" | "name" | "priority" | "definition">;
export type AutomationRun = { id: number; ruleId: number; ruleName: string; revision: number; conversationId: number; eventId: number; status: string; results: string[] | null; errorCode: string; attempts: number; createdAt: string; updatedAt: string; definition: AutomationDefinition };
const base = "/api/dashboard/automation";
const post = <T,>(path: string, body: unknown) => request<T>(base + path, { method: "POST", body: JSON.stringify(body) });
export const fetchAutomationRules = () => request<AutomationRule[]>(base + "/list");
export const saveAutomation = (draft: AutomationDraft) => post<AutomationRule>("/save", draft);
export const changeAutomation = (rule: AutomationRule, enabled: boolean) => post<void>("/change", { id: rule.id, revision: rule.revision, enabled });
export const deleteAutomation = (rule: AutomationRule) => post<void>("/delete", { id: rule.id, revision: rule.revision });
export const retryAutomation = (id: number) => post<void>("/retry", { id });
export const fetchAutomationRuns = (ruleId: number, page: number, limit: number) => request<{ results: AutomationRun[]; page: { page: number; limit: number; total: number } }>(`${base}/runs?ruleId=${ruleId}&page=${page}&limit=${limit}`);
export const testAutomation = (definition: AutomationDefinition, channel: string, text: string, leadTags: string[]) => post<{ matched: boolean; conditions: boolean[] }>("/test", { definition, channel, text, leadTags });
