import { CheckCircle2, Circle, Clock3, XCircle } from "lucide-react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type StepState = "complete" | "current" | "blocked" | "pending";

type Step = {
  label: string;
  detail: string;
  state: StepState;
};

const approvedStatuses = new Set(["approved", "provisioning", "completed"]);

function stepsFor(status: string): Step[] {
  const isRejected = status === "rejected";
  const isFailed = status === "failed";
  const isCompleted = status === "completed";
  const isProvisioning = status === "provisioning";
  const isPendingApproval = status === "pending_approval";

  return [
    {
      label: "Requested",
      detail: "Requester submitted the service request.",
      state: "complete",
    },
    {
      label: "Validation",
      detail: "OPORD accepted the request shape and routing context.",
      state: "complete",
    },
    {
      label: "Approval",
      detail: isRejected ? "Request was rejected by an approver." : "Governance policy decides whether provisioning can start.",
      state: isRejected ? "blocked" : isPendingApproval ? "current" : approvedStatuses.has(status) || isFailed ? "complete" : "pending",
    },
    {
      label: "Provisioning",
      detail: isFailed ? "Provisioning failed and needs operator attention." : "The selected provider creates or updates the service.",
      state: isFailed ? "blocked" : isCompleted ? "complete" : isProvisioning ? "current" : "pending",
    },
    {
      label: "Ownership",
      detail: "Resource is handed to the owner for operation, extension, or decommissioning.",
      state: isCompleted ? "complete" : "pending",
    },
  ];
}

function StepIcon({ state }: { state: StepState }) {
  if (state === "complete") return <CheckCircle2 className="size-5 text-emerald-400" />;
  if (state === "blocked") return <XCircle className="size-5 text-red-400" />;
  if (state === "current") return <Clock3 className="size-5 text-amber-400" />;
  return <Circle className="size-5 text-muted-foreground" />;
}

export function LifecycleTimeline({ status }: { status: string }) {
  const steps = stepsFor(status);

  return (
    <Card className="p-5">
      <div className="mb-4">
        <h2 className="text-sm font-semibold tracking-tight">Lifecycle</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Current request stage based on the backend status. Timestamps beyond creation are not available yet.
        </p>
      </div>
      <ol className="grid gap-3 md:grid-cols-5">
        {steps.map((step, index) => (
          <li key={step.label} className="relative">
            {index < steps.length - 1 && (
              <div className="absolute left-5 top-5 hidden h-px w-[calc(100%-1.25rem)] bg-border md:block" />
            )}
            <div className="relative z-10 flex gap-3 md:block">
              <div className="grid size-10 shrink-0 place-items-center rounded-lg border border-border bg-card">
                <StepIcon state={step.state} />
              </div>
              <div className="min-w-0 md:mt-3">
                <div
                  className={cn(
                    "text-sm font-semibold",
                    step.state === "pending" && "text-muted-foreground",
                    step.state === "blocked" && "text-red-300",
                    step.state === "current" && "text-amber-200",
                  )}
                >
                  {step.label}
                </div>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">{step.detail}</p>
              </div>
            </div>
          </li>
        ))}
      </ol>
    </Card>
  );
}
