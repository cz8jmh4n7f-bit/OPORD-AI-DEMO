"use client";

import { Download } from "lucide-react";
import { button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { AIAuditEvent } from "@/lib/types";

// csvCell escapes one CSV value (quotes doubled, wrapped when needed) and
// neutralizes spreadsheet formula injection (a cell starting with = + - @ would
// execute when the auditor opens the export in Excel/Sheets).
function csvCell(v: string): string {
  let s = v ?? "";
  if (/^[=+\-@]/.test(s)) s = "'" + s;
  if (/[",\n\r]/.test(s)) s = `"${s.replace(/"/g, '""')}"`;
  return s;
}

// AuditExportButton downloads the (already server-fetched) audit trail as CSV -
// the artifact an auditor asks for first. Client-side only: no new API surface,
// same RBAC as the page itself.
export function AuditExportButton({ events }: { events: AIAuditEvent[] }) {
  function exportCsv() {
    const header = ["time", "actor", "subject_type", "subject_id", "action", "message"];
    const rows = events.map((e) =>
      [e.createdAt, e.actor, e.subjectType, e.subjectId ?? "", e.action, e.message]
        .map((v) => csvCell(String(v)))
        .join(","),
    );
    const blob = new Blob([[header.join(","), ...rows].join("\r\n")], {
      type: "text/csv;charset=utf-8",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `opord-ai-audit-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <button
      type="button"
      onClick={exportCsv}
      disabled={events.length === 0}
      className={cn(button({ variant: "outline", size: "sm" }))}
      title="Download the audit trail as CSV"
    >
      <Download className="size-4" />
      Export CSV
    </button>
  );
}
