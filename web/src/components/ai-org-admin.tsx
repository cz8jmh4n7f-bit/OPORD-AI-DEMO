"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { createPortal } from "react-dom";
import { Archive, Gauge, KeyRound, Loader2, Plus, Settings2, ShieldCheck, Trash2, UserCog, UserPlus, Users } from "lucide-react";
import type {
  AIInvite,
  AIOrgUser,
  AIProjectAPIKey,
  AIProjectDataRetention,
  AIProjectHostedToolPermissions,
  AIProjectModelPermissions,
  AIProjectRateLimit,
  AIProjectSpendAlert,
  AIWorkspace,
  AIWorkspaceAccess,
} from "@/lib/types";
import { authHeaders } from "@/lib/client-auth";
import { button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { useConfirm } from "@/components/ui/confirm";
import { useToast } from "@/components/ui/toast";
import { cn } from "@/lib/utils";

const API = "/bff";
const inputCls =
  "h-9 w-full rounded-lg border border-input bg-card px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

// Org roles assignable via the API (admin is Console-only - mirrors the backend
// guard in the Anthropic provider). Workspace roles per the role matrix:
// workspace_billing is inherited-only, so it is not an assignable option.
// Role sets are provider-aware: Anthropic (org user/developer/billing… +
// workspace_*) vs OpenAI (org owner/reader + project owner/member).
const ROLE_SETS: Record<string, { org: string[]; ws: string[] }> = {
  anthropic: {
    org: ["user", "claude_code_user", "developer", "billing"],
    ws: ["workspace_user", "workspace_developer", "workspace_restricted_developer", "workspace_admin"],
  },
  openai: { org: ["reader", "owner"], ws: ["member", "owner"] },
};

function orgRolesFor(pt: string): string[] {
  return ROLE_SETS[pt]?.org ?? ROLE_SETS.anthropic.org;
}
function wsRolesFor(pt: string): string[] {
  return ROLE_SETS[pt]?.ws ?? ROLE_SETS.anthropic.ws;
}
// assignableWorkspaceRoles mirrors the backend matrix: an Anthropic billing user
// can ONLY become a workspace_admin; everyone else gets the provider's full set.
function assignableWorkspaceRoles(pt: string, orgRole: string): string[] {
  if (pt === "anthropic" && orgRole === "billing") return ["workspace_admin"];
  return wsRolesFor(pt);
}

function api(provider: string, path: string) {
  return `${API}/api/v1/ai/admin/${encodeURIComponent(provider)}${path}`;
}

// useAction wraps a mutation with busy state + toast + refresh, the pattern used
// across the AI action components.
function useAction() {
  const router = useRouter();
  const { toast } = useToast();
  const [busy, setBusy] = useState(false);
  async function run(label: string, fn: () => Promise<Response>): Promise<boolean> {
    setBusy(true);
    try {
      const res = await fn();
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        toast({ variant: "error", title: `${label} failed`, description: data.error ?? `Request failed (${res.status})` });
        return false;
      }
      toast({ variant: "success", title: `${label} done` });
      router.refresh();
      return true;
    } catch (err) {
      toast({ variant: "error", title: `${label} failed`, description: String(err) });
      return false;
    } finally {
      setBusy(false);
    }
  }
  return { busy, run };
}

function roleLabel(role: string) {
  return role.replace(/_/g, " ");
}

// --- Org users panel ---

export function OrgUsersPanel({ provider, providerType, users }: { provider: string; providerType: string; users: AIOrgUser[] }) {
  return (
    <Card className="overflow-hidden p-0">
      <div className="flex items-center justify-between border-b border-border px-5 py-3">
        <div className="flex items-center gap-2">
          <Users className="size-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Organization users ({users.length})</h2>
        </div>
        <InviteUserButton provider={provider} providerType={providerType} />
      </div>
      {users.length === 0 ? (
        <p className="px-5 py-8 text-center text-sm text-muted-foreground">No users yet. Invite one to get started.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th scope="col" className="px-5 py-3 font-medium">User</th>
                <th scope="col" className="px-5 py-3 font-medium">Org role</th>
                <th scope="col" className="px-5 py-3 font-medium">Added</th>
                <th scope="col" className="px-5 py-3 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b border-border last:border-0 hover:bg-muted/60">
                  <td className="px-5 py-3">
                    <div className="font-medium text-foreground">{u.email}</div>
                    {u.name && <div className="text-xs text-muted-foreground">{u.name}</div>}
                  </td>
                  <td className="px-5 py-3">
                    <Badge>{roleLabel(u.role)}</Badge>
                  </td>
                  <td className="px-5 py-3 text-muted-foreground">{u.addedAt ? u.addedAt.slice(0, 10) : "-"}</td>
                  <td className="px-5 py-3">
                    <OrgUserActions provider={provider} providerType={providerType} user={u} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

function InviteUserButton({ provider, providerType }: { provider: string; providerType: string }) {
  const { busy, run } = useAction();
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState(orgRolesFor(providerType)[0]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const ok = await run("Invite", () =>
      fetch(api(provider, "/invites"), {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ email, role }),
      }),
    );
    if (ok) {
      setOpen(false);
      setEmail("");
    }
  }

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className={button({ variant: "outline", size: "sm" })}>
        <UserPlus className="size-4" />
        Invite user
      </button>
      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <Modal title="Invite organization user" onClose={() => !busy && setOpen(false)}>
            <form onSubmit={submit} className="space-y-4">
              <p className="text-xs text-muted-foreground">
                Sends a pending invite. The user is added to the organization only after they accept it (the invite can
                expire unaccepted).
              </p>
              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Email</span>
                <input className={inputCls} type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="person@company.com" />
              </label>
              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Org role</span>
                <select className={inputCls} value={role} onChange={(e) => setRole(e.target.value)}>
                  {orgRolesFor(providerType).map((r) => (
                    <option key={r} value={r}>{roleLabel(r)}</option>
                  ))}
                </select>
                <span className="text-xs text-muted-foreground">The admin role can only be granted in the provider Console.</span>
              </label>
              <ModalActions busy={busy} onCancel={() => setOpen(false)} submitLabel="Send invite" />
            </form>
          </Modal>,
          document.body,
        )}
    </>
  );
}

function OrgUserActions({ provider, providerType, user }: { provider: string; providerType: string; user: AIOrgUser }) {
  const { busy, run } = useAction();
  const { prompt } = useConfirm();
  const [open, setOpen] = useState(false);
  const [role, setRole] = useState(user.role);

  async function saveRole(e: React.FormEvent) {
    e.preventDefault();
    const ok = await run("Role change", () =>
      fetch(api(provider, `/users/${encodeURIComponent(user.id)}/role`), {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ role }),
      }),
    );
    if (ok) setOpen(false);
  }

  async function remove() {
    const typed = await prompt({
      title: `Remove ${user.email}?`,
      message: "Removes the user from the organization (this does not delete their personal account). Type the email to confirm.",
      requireValue: user.email,
      confirmLabel: "Remove",
      danger: true,
    });
    if (typed == null) return;
    await run("Remove user", () =>
      fetch(api(provider, `/users/${encodeURIComponent(user.id)}`), { method: "DELETE", headers: authHeaders() }),
    );
  }

  return (
    <div className="flex justify-end gap-2">
      <button type="button" onClick={() => setOpen(true)} disabled={busy} className={button({ variant: "outline", size: "sm" })}>
        <UserCog className="size-4" />
        Role
      </button>
      <button type="button" onClick={remove} disabled={busy} className={cn(button({ variant: "outline", size: "sm" }), "text-danger")}>
        {busy ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
        Remove
      </button>
      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <Modal title={`Change role · ${user.email}`} onClose={() => !busy && setOpen(false)}>
            <form onSubmit={saveRole} className="space-y-4">
              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Org role</span>
                <select className={inputCls} value={role} onChange={(e) => setRole(e.target.value)}>
                  {orgRolesFor(providerType).map((r) => (
                    <option key={r} value={r}>{roleLabel(r)}</option>
                  ))}
                </select>
              </label>
              <ModalActions busy={busy} onCancel={() => setOpen(false)} submitLabel="Save role" />
            </form>
          </Modal>,
          document.body,
        )}
    </div>
  );
}

// --- Invites panel ---

export function InvitesPanel({ invites }: { invites: AIInvite[] }) {
  const pending = invites.filter((i) => i.status === "pending");
  if (pending.length === 0) return null;
  return (
    <Card className="overflow-hidden p-0">
      <div className="border-b border-border px-5 py-3">
        <h2 className="text-sm font-semibold">Pending invites ({pending.length})</h2>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-muted-foreground">
              <th scope="col" className="px-5 py-3 font-medium">Email</th>
              <th scope="col" className="px-5 py-3 font-medium">Role</th>
              <th scope="col" className="px-5 py-3 font-medium">Expires</th>
            </tr>
          </thead>
          <tbody>
            {pending.map((i) => (
              <tr key={i.inviteId} className="border-b border-border last:border-0">
                <td className="px-5 py-3 font-medium">{i.email}</td>
                <td className="px-5 py-3"><Badge>{roleLabel(i.role)}</Badge></td>
                <td className="px-5 py-3 text-muted-foreground">{i.expiresAt ? i.expiresAt.slice(0, 10) : "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

// --- Workspaces panel ---

export function WorkspacesPanel({ provider, providerType, workspaces, users }: { provider: string; providerType: string; workspaces: AIWorkspace[]; users: AIOrgUser[] }) {
  const active = workspaces.filter((w) => !w.archived);
  return (
    <Card className="overflow-hidden p-0">
      <div className="flex items-center justify-between border-b border-border px-5 py-3">
        <h2 className="text-sm font-semibold">Workspaces ({active.length})</h2>
        <CreateWorkspaceButton provider={provider} />
      </div>
      {active.length === 0 ? (
        <p className="px-5 py-8 text-center text-sm text-muted-foreground">No active workspaces. Create one to scope access.</p>
      ) : (
        <div className="divide-y divide-border">
          {active.map((w) => (
            <WorkspaceRow key={w.id} provider={provider} providerType={providerType} workspace={w} users={users} />
          ))}
        </div>
      )}
    </Card>
  );
}

function CreateWorkspaceButton({ provider }: { provider: string }) {
  const { busy, run } = useAction();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const ok = await run("Create workspace", () =>
      fetch(api(provider, "/workspaces"), {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ name }),
      }),
    );
    if (ok) {
      setOpen(false);
      setName("");
    }
  }

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className={button({ variant: "outline", size: "sm" })}>
        <Plus className="size-4" />
        New workspace
      </button>
      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <Modal title="Create workspace" onClose={() => !busy && setOpen(false)}>
            <form onSubmit={submit} className="space-y-4">
              <label className="flex flex-col gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">Name</span>
                <input className={inputCls} required value={name} onChange={(e) => setName(e.target.value)} placeholder="team-platform" />
              </label>
              <ModalActions busy={busy} onCancel={() => setOpen(false)} submitLabel="Create" />
            </form>
          </Modal>,
          document.body,
        )}
    </>
  );
}

function WorkspaceRow({ provider, providerType, workspace, users }: { provider: string; providerType: string; workspace: AIWorkspace; users: AIOrgUser[] }) {
  const { busy, run } = useAction();
  const { prompt } = useConfirm();
  const [open, setOpen] = useState(false);

  async function archive() {
    const typed = await prompt({
      title: `Archive ${workspace.name}?`,
      message: "Archiving retires the workspace (the provider has no hard delete). Type the workspace name to confirm.",
      requireValue: workspace.name,
      confirmLabel: "Archive",
      danger: true,
    });
    if (typed == null) return;
    await run("Archive workspace", () =>
      fetch(api(provider, `/workspaces/${encodeURIComponent(workspace.id)}/archive`), { method: "POST", headers: authHeaders() }),
    );
  }

  return (
    <div className="px-5 py-3">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate font-medium text-foreground">{workspace.name}</div>
          <div className="font-mono text-xs text-muted-foreground">{workspace.id}</div>
        </div>
        <div className="flex shrink-0 gap-2">
          <button type="button" onClick={() => setOpen((v) => !v)} disabled={busy} className={button({ variant: "outline", size: "sm" })}>
            <Users className="size-4" />
            Members
          </button>
          <button type="button" onClick={archive} disabled={busy} className={cn(button({ variant: "outline", size: "sm" }), "text-danger")}>
            {busy ? <Loader2 className="size-4 animate-spin" /> : <Archive className="size-4" />}
            Archive
          </button>
        </div>
      </div>
      {open && (
        <div className="mt-3 space-y-3">
          <WorkspaceMembers provider={provider} providerType={providerType} workspace={workspace} users={users} />
          {providerType === "openai" && <OpenAIProjectControls provider={provider} workspace={workspace} />}
        </div>
      )}
    </div>
  );
}

// WorkspaceMembers lazily loads the EFFECTIVE access set (explicit + inherited)
// when expanded, and offers grant/remove. Inherited rows (org admin/billing) are
// shown read-only since they come from the org role, not a workspace membership.
function WorkspaceMembers({ provider, providerType, workspace, users }: { provider: string; providerType: string; workspace: AIWorkspace; users: AIOrgUser[] }) {
  const { busy, run } = useAction();
  const [access, setAccess] = useState<AIWorkspaceAccess[] | null>(null);
  const [userId, setUserId] = useState("");
  const [role, setRole] = useState("workspace_user");
  // The selected user's org role decides which workspace roles are valid (matrix).
  const selectedUser = users.find((u) => u.id === userId);
  const roleOptions = assignableWorkspaceRoles(providerType, selectedUser?.role ?? "");
  // Org admins inherit workspace_admin everywhere - they can't (and needn't) be
  // granted explicitly, so they're not offered in the picker.
  const grantable = users.filter((u) => u.role !== "admin");

  // Fetch the effective access set on mount + after mutations. The setState lives
  // inside the async body (deferred past the first await), mirroring the codebase's
  // fetch-on-mount pattern so the strict set-state-in-effect rule stays satisfied.
  const load = useCallback(() => {
    void (async () => {
      try {
        const res = await fetch(api(provider, `/workspaces/${encodeURIComponent(workspace.id)}/access`), { headers: authHeaders() });
        const data = await res.json().catch(() => []);
        setAccess(res.ok && Array.isArray(data) ? data : []);
      } catch {
        setAccess([]);
      }
    })();
  }, [provider, workspace.id]);

  useEffect(() => {
    load();
  }, [load]);

  async function grant(e: React.FormEvent) {
    e.preventDefault();
    const ok = await run("Grant access", () =>
      fetch(api(provider, `/workspaces/${encodeURIComponent(workspace.id)}/members`), {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ userId, role }),
      }),
    );
    if (ok) {
      setUserId("");
      load();
    }
  }

  async function removeMember(uid: string) {
    const ok = await run("Remove member", () =>
      fetch(api(provider, `/workspaces/${encodeURIComponent(workspace.id)}/members/${encodeURIComponent(uid)}`), { method: "DELETE", headers: authHeaders() }),
    );
    if (ok) load();
  }

  return (
    <div className="mt-3 space-y-3 rounded-lg border border-border bg-muted/30 p-3">
      {access === null && <p className="text-xs text-muted-foreground">Loading members…</p>}
      {access && access.length === 0 && <p className="text-xs text-muted-foreground">No members yet.</p>}
      {access && access.length > 0 && (
        <ul className="space-y-1.5">
          {access.map((a) => (
            <li key={a.userId} className="flex items-center justify-between gap-3 text-sm">
              <span className="min-w-0 truncate">
                {a.email || a.userId}
                <Badge className="ml-2">{roleLabel(a.workspaceRole)}</Badge>
                {a.inherited && <span className="ml-2 text-xs text-muted-foreground">inherited from {roleLabel(a.orgRole || "org role")}</span>}
              </span>
              {!a.inherited && (
                <button type="button" onClick={() => removeMember(a.userId)} disabled={busy} className="text-xs font-medium text-danger hover:underline">
                  Remove
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      <form onSubmit={grant} className="flex flex-wrap items-end gap-2 border-t border-border pt-3">
        <label className="flex flex-1 flex-col gap-1">
          <span className="text-xs font-medium text-muted-foreground">User</span>
          <select
            className={inputCls}
            required
            value={userId}
            onChange={(e) => {
              const next = e.target.value;
              setUserId(next);
              // Snap the role into the matrix-allowed set for the chosen user.
              const allowed = assignableWorkspaceRoles(providerType, users.find((u) => u.id === next)?.role ?? "");
              if (!allowed.includes(role)) setRole(allowed[0]);
            }}
          >
            <option value="" disabled>
              Select a user…
            </option>
            {grantable.map((u) => (
              <option key={u.id} value={u.id}>
                {u.email} ({roleLabel(u.role)})
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs font-medium text-muted-foreground">Role</span>
          <select className={inputCls} value={role} onChange={(e) => setRole(e.target.value)} disabled={!userId}>
            {roleOptions.map((r) => (
              <option key={r} value={r}>{roleLabel(r)}</option>
            ))}
          </select>
        </label>
        <button type="submit" disabled={busy || !userId} className={button({ size: "sm" })}>
          {busy ? <Loader2 className="size-4 animate-spin" /> : <UserPlus className="size-4" />}
          Grant
        </button>
      </form>
      {selectedUser?.role === "billing" && (
        <p className="text-xs text-muted-foreground">
          {selectedUser.email} has the <span className="font-medium">billing</span> org role - they already see billing on
          every workspace and the provider only allows promoting them to <span className="font-medium">workspace admin</span>.
          To make them a regular developer/user, change their org role first.
        </p>
      )}
      {grantable.length === 0 && (
        <p className="text-xs text-muted-foreground">No grantable users - invite one, or org admins already have inherited access.</p>
      )}
    </div>
  );
}

function OpenAIProjectControls({ provider, workspace }: { provider: string; workspace: AIWorkspace }) {
  const { busy, run } = useAction();
  const { prompt, confirm } = useConfirm();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [apiKeys, setAPIKeys] = useState<AIProjectAPIKey[]>([]);
  const [rateLimits, setRateLimits] = useState<AIProjectRateLimit[]>([]);
  const [modelPerms, setModelPerms] = useState<AIProjectModelPermissions>({ mode: "allow_list", modelIds: [] });
  const [toolPerms, setToolPerms] = useState<AIProjectHostedToolPermissions>({
    codeInterpreter: false,
    fileSearch: false,
    imageGeneration: false,
    mcp: false,
    webSearch: false,
  });
  const [retention, setRetention] = useState<AIProjectDataRetention>({ type: "organization_default" });
  const [alerts, setAlerts] = useState<AIProjectSpendAlert[]>([]);
  const [modelText, setModelText] = useState("");
  const [alertUSD, setAlertUSD] = useState("");
  const [alertRecipients, setAlertRecipients] = useState("");

  const base = api(provider, `/workspaces/${encodeURIComponent(workspace.id)}`);

  const load = useCallback(() => {
    void (async () => {
      setLoading(true);
      setError("");
      try {
        const [keysRes, limitsRes, modelsRes, toolsRes, retentionRes, alertsRes] = await Promise.all([
          fetch(`${base}/api-keys`, { headers: authHeaders() }),
          fetch(`${base}/rate-limits`, { headers: authHeaders() }),
          fetch(`${base}/model-permissions`, { headers: authHeaders() }),
          fetch(`${base}/tool-permissions`, { headers: authHeaders() }),
          fetch(`${base}/data-retention`, { headers: authHeaders() }),
          fetch(`${base}/spend-alerts`, { headers: authHeaders() }),
        ]);
        const firstBad = [keysRes, limitsRes, modelsRes, toolsRes, retentionRes, alertsRes].find((r) => !r.ok);
        if (firstBad) {
          const data = await firstBad.json().catch(() => ({}));
          throw new Error(data.error ?? `OpenAI controls unavailable (${firstBad.status})`);
        }
        const nextModels = (await modelsRes.json()) as AIProjectModelPermissions;
        setAPIKeys((await keysRes.json()) as AIProjectAPIKey[]);
        setRateLimits((await limitsRes.json()) as AIProjectRateLimit[]);
        setModelPerms(nextModels);
        setModelText((nextModels.modelIds ?? []).join(", "));
        setToolPerms((await toolsRes.json()) as AIProjectHostedToolPermissions);
        setRetention((await retentionRes.json()) as AIProjectDataRetention);
        setAlerts((await alertsRes.json()) as AIProjectSpendAlert[]);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    })();
  }, [base]);

  useEffect(() => {
    load();
  }, [load]);

  async function saveModels(e: React.FormEvent) {
    e.preventDefault();
    const modelIds = modelText.split(",").map((m) => m.trim()).filter(Boolean);
    const ok = await run("Save model permissions", () =>
      fetch(`${base}/model-permissions`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ mode: modelPerms.mode, modelIds }),
      }),
    );
    if (ok) load();
  }

  async function clearModels() {
    const typed = await prompt({
      title: "Clear model permissions?",
      message: `This removes the model allowlist/denylist on ${workspace.name}. Type the project name to confirm.`,
      requireValue: workspace.name,
      confirmLabel: "Clear",
      danger: true,
    });
    if (typed == null) return;
    const ok = await run("Clear model permissions", () =>
      fetch(`${base}/model-permissions`, { method: "DELETE", headers: authHeaders() }),
    );
    if (ok) load();
  }

  async function saveTools(e: React.FormEvent) {
    e.preventDefault();
    const ok = await run("Save tool permissions", () =>
      fetch(`${base}/tool-permissions`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify(toolPerms),
      }),
    );
    if (ok) load();
  }

  async function saveRetention(e: React.FormEvent) {
    e.preventDefault();
    // Data retention is a compliance-relevant policy change; confirm before the
    // one-click apply so it isn't changed by accident.
    if (
      !(await confirm({
        title: "Change data retention?",
        message: `Set data retention to "${retention.type}" on ${workspace.name}? This changes how long OpenAI keeps this project's data.`,
        danger: true,
        confirmLabel: "Apply",
      }))
    )
      return;
    const ok = await run("Save data retention", () =>
      fetch(`${base}/data-retention`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify(retention),
      }),
    );
    if (ok) load();
  }

  async function deleteKey(key: AIProjectAPIKey) {
    const typed = await prompt({
      title: `Delete API key ${key.name || key.id}?`,
      message: "This permanently deletes the OpenAI project API key. Type the key id to confirm.",
      requireValue: key.id,
      confirmLabel: "Delete",
      danger: true,
    });
    if (typed == null) return;
    const ok = await run("Delete API key", () =>
      fetch(`${base}/api-keys/${encodeURIComponent(key.id)}`, { method: "DELETE", headers: authHeaders() }),
    );
    if (ok) load();
  }

  async function createAlert(e: React.FormEvent) {
    e.preventDefault();
    const recipients = alertRecipients.split(",").map((x) => x.trim()).filter(Boolean);
    const ok = await run("Create spend alert", () =>
      fetch(`${base}/spend-alerts`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ thresholdUsd: Number(alertUSD), recipients }),
      }),
    );
    if (ok) {
      setAlertUSD("");
      setAlertRecipients("");
      load();
    }
  }

  async function deleteAlert(alert: AIProjectSpendAlert) {
    const typed = await prompt({
      title: "Delete spend alert?",
      message: "Type the alert id to confirm.",
      requireValue: alert.id,
      confirmLabel: "Delete",
      danger: true,
    });
    if (typed == null) return;
    const ok = await run("Delete spend alert", () =>
      fetch(`${base}/spend-alerts/${encodeURIComponent(alert.id)}`, { method: "DELETE", headers: authHeaders() }),
    );
    if (ok) load();
  }

  if (loading) {
    return <div className="rounded-lg border border-border bg-muted/30 p-3 text-xs text-muted-foreground">Loading OpenAI project controls...</div>;
  }

  if (error) {
    return (
      <div className="rounded-lg border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
        OpenAI project controls unavailable: {error}
      </div>
    );
  }

  return (
    <div className="space-y-4 rounded-lg border border-border bg-muted/30 p-3">
      <div className="flex items-center gap-2">
        <Settings2 className="size-4 text-muted-foreground" />
        <h3 className="text-sm font-semibold text-foreground">OpenAI project controls</h3>
      </div>

      <section className="grid gap-3 lg:grid-cols-2">
        <form onSubmit={saveModels} className="space-y-2 rounded-lg border border-border bg-card p-3">
          <ControlTitle icon={ShieldCheck} title="Model permissions" />
          <select className={inputCls} value={modelPerms.mode || "allow_list"} onChange={(e) => setModelPerms({ ...modelPerms, mode: e.target.value })}>
            <option value="allow_list">Allow list</option>
            <option value="deny_list">Deny list</option>
          </select>
          <textarea
            className="min-h-20 w-full rounded-lg border border-input bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={modelText}
            onChange={(e) => setModelText(e.target.value)}
            placeholder="gpt-4.1, o4-mini"
          />
          <div className="flex justify-end gap-2">
            <button type="button" onClick={clearModels} disabled={busy} className={button({ variant: "outline", size: "sm" })}>Clear</button>
            <button type="submit" disabled={busy} className={button({ size: "sm" })}>Save</button>
          </div>
        </form>

        <form onSubmit={saveTools} className="space-y-2 rounded-lg border border-border bg-card p-3">
          <ControlTitle icon={Settings2} title="Hosted tools" />
          <div className="grid grid-cols-2 gap-2 text-sm">
            {[
              ["webSearch", "Web search"],
              ["fileSearch", "File search"],
              ["codeInterpreter", "Code interpreter"],
              ["imageGeneration", "Image generation"],
              ["mcp", "MCP"],
            ].map(([key, label]) => (
              <label key={key} className="flex items-center gap-2 rounded-md border border-border px-2 py-1.5">
                <input
                  type="checkbox"
                  checked={Boolean(toolPerms[key as keyof AIProjectHostedToolPermissions])}
                  onChange={(e) => setToolPerms({ ...toolPerms, [key]: e.target.checked })}
                />
                {label}
              </label>
            ))}
          </div>
          <div className="flex justify-end">
            <button type="submit" disabled={busy} className={button({ size: "sm" })}>Save tools</button>
          </div>
        </form>
      </section>

      <section className="grid gap-3 lg:grid-cols-2">
        <form onSubmit={saveRetention} className="space-y-2 rounded-lg border border-border bg-card p-3">
          <ControlTitle icon={ShieldCheck} title="Data retention" />
          <select className={inputCls} value={retention.type || "organization_default"} onChange={(e) => setRetention({ type: e.target.value })}>
            <option value="organization_default">Organization default</option>
            <option value="none">None</option>
            <option value="zero_data_retention">Zero data retention</option>
            <option value="modified_abuse_monitoring">Modified abuse monitoring</option>
            <option value="enhanced_zero_data_retention">Enhanced zero data retention</option>
            <option value="enhanced_modified_abuse_monitoring">Enhanced modified abuse monitoring</option>
          </select>
          <div className="flex justify-end">
            <button type="submit" disabled={busy} className={button({ size: "sm" })}>Save retention</button>
          </div>
        </form>

        <div className="space-y-2 rounded-lg border border-border bg-card p-3">
          <ControlTitle icon={KeyRound} title={`API keys (${apiKeys.length})`} />
          {apiKeys.length === 0 ? (
            <p className="text-xs text-muted-foreground">No project API keys reported.</p>
          ) : (
            <ul className="space-y-1.5">
              {apiKeys.map((key) => (
                <li key={key.id} className="flex items-center justify-between gap-2 text-sm">
                  <span className="min-w-0 truncate">
                    <span className="font-medium">{key.name || key.id}</span>
                    <span className="ml-2 font-mono text-xs text-muted-foreground">{key.redactedValue}</span>
                  </span>
                  <button type="button" onClick={() => deleteKey(key)} disabled={busy} className="text-xs font-medium text-danger hover:underline">
                    Delete
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>

      <section className="grid gap-3 lg:grid-cols-2">
        <div className="space-y-2 rounded-lg border border-border bg-card p-3">
          <ControlTitle icon={Gauge} title={`Rate limits (${rateLimits.length})`} />
          {rateLimits.length === 0 ? (
            <p className="text-xs text-muted-foreground">No rate limits reported.</p>
          ) : (
            <div className="max-h-56 overflow-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-left text-muted-foreground">
                    <th className="py-1 font-medium">Model</th>
                    <th className="py-1 font-medium">RPM</th>
                    <th className="py-1 font-medium">TPM</th>
                  </tr>
                </thead>
                <tbody>
                  {rateLimits.map((limit) => (
                    <tr key={limit.id} className="border-t border-border">
                      <td className="py-1 font-mono">{limit.model}</td>
                      <td className="py-1">{limit.maxRequestsPer1Minute || "-"}</td>
                      <td className="py-1">{limit.maxTokensPer1Minute || "-"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        <div className="space-y-2 rounded-lg border border-border bg-card p-3">
          <ControlTitle icon={Gauge} title={`Spend alerts (${alerts.length})`} />
          {alerts.length > 0 && (
            <ul className="space-y-1.5">
              {alerts.map((alert) => (
                <li key={alert.id} className="flex items-center justify-between gap-2 text-sm">
                  <span>
                    ${(alert.thresholdCents / 100).toLocaleString()} {alert.recipients?.length ? `to ${alert.recipients.join(", ")}` : ""}
                  </span>
                  <button type="button" onClick={() => deleteAlert(alert)} disabled={busy} className="text-xs font-medium text-danger hover:underline">
                    Delete
                  </button>
                </li>
              ))}
            </ul>
          )}
          <form onSubmit={createAlert} className="grid grid-cols-[1fr_2fr_auto] gap-2 border-t border-border pt-2">
            <input className={inputCls} type="number" min="1" step="1" value={alertUSD} onChange={(e) => setAlertUSD(e.target.value)} placeholder="USD" required />
            <input className={inputCls} value={alertRecipients} onChange={(e) => setAlertRecipients(e.target.value)} placeholder="emails, comma-separated" />
            <button type="submit" disabled={busy} className={button({ size: "sm" })}>Add</button>
          </form>
        </div>
      </section>
    </div>
  );
}

function ControlTitle({ icon: Icon, title }: { icon: React.ComponentType<{ className?: string }>; title: string }) {
  return (
    <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
      <Icon className="size-4 text-muted-foreground" />
      {title}
    </div>
  );
}

// --- Shared modal chrome ---

function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center p-4" role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative w-full max-w-md rounded-xl border border-border bg-card p-5 shadow-xl">
        <h2 className="mb-4 text-base font-semibold text-foreground">{title}</h2>
        {children}
      </div>
    </div>
  );
}

function ModalActions({ busy, onCancel, submitLabel }: { busy: boolean; onCancel: () => void; submitLabel: string }) {
  return (
    <div className="flex items-center justify-end gap-2 pt-1">
      <button type="button" onClick={onCancel} disabled={busy} className={button({ variant: "outline", size: "sm" })}>
        Cancel
      </button>
      <button type="submit" disabled={busy} className={cn(button({ size: "sm" }), busy && "opacity-70")}>
        {busy && <Loader2 className="size-4 animate-spin" />}
        {submitLabel}
      </button>
    </div>
  );
}
