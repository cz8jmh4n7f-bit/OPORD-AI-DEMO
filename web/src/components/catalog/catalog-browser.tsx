"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import {
  Archive,
  BadgeCheck,
  Blocks,
  Boxes,
  Building2,
  Cloud,
  Cpu,
  Database,
  Gauge,
  Globe,
  Globe2,
  Inbox,
  KeyRound,
  Layers,
  Lock,
  Network,
  Search,
  Server,
  Table2,
  UserCheck,
  Webhook,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { Provider } from "@/lib/types";
import { cn } from "@/lib/utils";

type CatalogCategory =
  | "Compute"
  | "Data"
  | "App services"
  | "Security & Access"
  | "Environments"
  | "Landing zones"
  | "Automation";

type CatalogItem = {
  label: string;
  desc: string;
  icon: LucideIcon;
  href: string;
  provider: Provider;
  via?: string;
  category: CatalogCategory;
  tags: string[];
  ttlSupport?: boolean;
  approval?: "Policy" | "Auto";
  costHint?: string;
};

const categories: Array<"All" | CatalogCategory> = [
  "All",
  "Compute",
  "Data",
  "App services",
  "Security & Access",
  "Environments",
  "Landing zones",
  "Automation",
];

const providerVisuals: Record<string, { label: string; icon: LucideIcon; accent: string }> = {
  aws: {
    label: "AWS",
    icon: Cloud,
    accent: "border-amber-400/20 bg-amber-500/10 text-amber-300",
  },
  azure: {
    label: "Azure",
    icon: Cloud,
    accent: "border-sky-400/20 bg-sky-500/10 text-sky-300",
  },
  gcp: {
    label: "GCP",
    icon: Cloud,
    accent: "border-emerald-400/20 bg-emerald-500/10 text-emerald-300",
  },
  proxmox: {
    label: "Proxmox",
    icon: Server,
    accent: "border-orange-400/20 bg-orange-500/10 text-orange-300",
  },
  vsphere: {
    label: "vSphere",
    icon: Boxes,
    accent: "border-slate-400/20 bg-slate-500/10 text-slate-300",
  },
};

function providerVisual(type: string) {
  return providerVisuals[type] ?? {
    label: type,
    icon: Server,
    accent: "border-border bg-muted text-muted-foreground",
  };
}

function commonItems(type: string, provider: Provider): CatalogItem[] {
  const p = encodeURIComponent(provider.name);
  const vmDesc =
    type === "aws"
      ? "EC2 instance with standardized sizing and ownership metadata."
      : type === "azure"
        ? "Linux VM with provider safety profile and public access guardrails."
        : type === "gcp"
          ? "Compute Engine VM in the configured project, region, and zone."
          : "VM sized by vCPU, RAM, disk, and optional TTL.";
  return [
    {
      label: "Virtual machine",
      desc: vmDesc,
      icon: Cpu,
      href: `/vms/new?provider=${p}`,
      provider,
      via: type === "aws" ? "EC2" : type === "azure" ? "Azure VM" : type === "gcp" ? "Compute Engine" : undefined,
      category: "Compute",
      tags: ["VM", "Compute", "TTL"],
      ttlSupport: true,
      approval: "Policy",
      costHint: "Estimate before submit",
    },
    {
      label: "Kubernetes cluster",
      desc:
        type === "aws"
          ? "Managed EKS control plane and node pool."
          : type === "azure"
            ? "Managed AKS control plane and node pool."
            : type === "gcp"
              ? "Managed GKE control plane and node pool."
              : "kubeadm cluster across provisioned VMs.",
      icon: Boxes,
      href: `/clusters/new?provider=${p}`,
      provider,
      via: type === "aws" ? "EKS" : type === "azure" ? "AKS" : type === "gcp" ? "GKE" : undefined,
      category: "Compute",
      tags: ["Kubernetes", "Cluster"],
      approval: "Policy",
      costHint: "Estimate before submit",
    },
  ];
}

function itemsForProvider(provider: Provider): CatalogItem[] {
  const type = provider.type;
  const p = encodeURIComponent(provider.name);
  const base = commonItems(type, provider);

  if (type === "vsphere" || type === "proxmox") return base;

  const cloudData: CatalogItem[] = [
    {
      label: "Database",
      desc:
        type === "aws"
          ? "Managed RDS database with standardized network and backup defaults."
          : type === "azure"
            ? "PostgreSQL Flexible Server with private-ready guardrails."
            : "Cloud SQL database with provider safety defaults.",
      icon: Database,
      href: `/databases/new?provider=${p}`,
      provider,
      via: type === "aws" ? "RDS" : type === "azure" ? "PostgreSQL" : "Cloud SQL",
      category: "Data",
      tags: ["Database", "PostgreSQL", "Backup"],
      approval: "Policy",
      costHint: "Estimate before submit",
    },
    {
      label: "Object storage",
      desc:
        type === "aws"
          ? "Private versioned S3 bucket."
          : type === "azure"
            ? "Blob storage account with public access guardrails."
            : "Cloud Storage bucket with uniform access and public access prevention.",
      icon: Archive,
      href: `/s3/new?provider=${p}`,
      provider,
      via: type === "aws" ? "S3" : type === "azure" ? "Storage" : "Cloud Storage",
      category: "Data",
      tags: ["Storage", "Bucket"],
      approval: "Auto",
      costHint: "Low baseline cost",
    },
    {
      label: "Cache",
      desc:
        type === "aws"
          ? "Managed Redis via ElastiCache."
          : type === "azure"
            ? "Managed Redis via Azure Cache."
            : "Memorystore for Redis with single-zone or HA mode.",
      icon: Gauge,
      href: `/caches/new?provider=${p}`,
      provider,
      via: type === "aws" ? "ElastiCache" : type === "azure" ? "Redis" : "Memorystore",
      category: "Data",
      tags: ["Redis", "Cache"],
      approval: "Policy",
      costHint: "Estimate before submit",
    },
    {
      label: "Table",
      desc:
        type === "aws"
          ? "DynamoDB table with on-demand or provisioned capacity."
          : type === "azure"
            ? "Cosmos DB SQL container with serverless or RU capacity."
            : "Firestore database for managed NoSQL workloads.",
      icon: Table2,
      href: `/tables/new?provider=${p}`,
      provider,
      via: type === "aws" ? "DynamoDB" : type === "azure" ? "Cosmos DB" : "Firestore",
      category: "Data",
      tags: ["NoSQL", "Table"],
      approval: "Policy",
      costHint: "Estimate before submit",
    },
  ];

  const appServices: CatalogItem[] = [
    {
      label: "Function",
      desc:
        type === "aws"
          ? "Serverless Lambda function."
          : type === "azure"
            ? "Linux Function App with standardized runtime."
            : "2nd-generation Cloud Function with managed build path.",
      icon: Zap,
      href: `/functions/new?provider=${p}`,
      provider,
      via: type === "aws" ? "Lambda" : type === "azure" ? "Functions" : "Cloud Functions",
      category: "App services",
      tags: ["Serverless", "Function"],
      approval: "Auto",
      costHint: "Usage-based",
    },
    {
      label: "Queue",
      desc:
        type === "aws"
          ? "SQS queue, standard or FIFO."
          : type === "azure"
            ? "Service Bus namespace and queue."
            : "Pub/Sub topic and pull subscription.",
      icon: Inbox,
      href: `/queues/new?provider=${p}`,
      provider,
      via: type === "aws" ? "SQS" : type === "azure" ? "Service Bus" : "Pub/Sub",
      category: "App services",
      tags: ["Queue", "Messaging"],
      approval: "Auto",
      costHint: "Usage-based",
    },
  ];

  const accessItems: CatalogItem[] = [
    {
      label: "Secret namespace",
      desc:
        type === "aws"
          ? "Secrets Manager secret container."
          : type === "azure"
            ? "Key Vault with safety-profile retention defaults."
            : "Secret Manager container; value set out-of-band.",
      icon: Lock,
      href: `/secrets/new?provider=${p}`,
      provider,
      via: type === "aws" ? "Secrets Manager" : type === "azure" ? "Key Vault" : "Secret Manager",
      category: "Security & Access",
      tags: ["Secret", "Security"],
      approval: "Policy",
      costHint: "Low baseline cost",
    },
    {
      label: "Project access",
      desc:
        type === "aws"
          ? "Identity Center group and account access."
          : type === "azure"
            ? "Entra group granted an Azure RBAC role."
            : "Grant IAM role access on a GCP project.",
      icon: KeyRound,
      href: `/projects/new?provider=${p}`,
      provider,
      via: type === "aws" ? "IAM Identity Center" : type === "azure" ? "Entra + RBAC" : "IAM",
      category: "Security & Access",
      tags: ["Access", "RBAC"],
      approval: "Policy",
    },
  ];

  const landingZones: CatalogItem[] = [
    {
      label: type === "azure" ? "Managed subscription" : type === "gcp" ? "Managed project" : "Managed account",
      desc:
        type === "azure"
          ? "Governed subscription baseline with RBAC, VNet, logs, and policy."
          : type === "gcp"
            ? "Isolated GCP project with APIs, KMS, secure VPC, org policies, and IAM."
            : "New member account and security baseline.",
      icon: Building2,
      href: `/accounts/new?provider=${p}`,
      provider,
      via: type === "azure" ? "Landing zone" : type === "gcp" ? "Project factory" : "Organizations",
      category: "Landing zones",
      tags: ["Landing zone", "Account"],
      approval: "Policy",
      costHint: "Container; child resources bill separately",
    },
  ];

  const automation: CatalogItem[] = [
    {
      label: "OpenTofu stack",
      desc: `Run an approved ${provider.type.toUpperCase()} OpenTofu root module for advanced services.`,
      icon: Blocks,
      href: `/stacks/new?provider=${p}`,
      provider,
      category: "Automation",
      tags: ["OpenTofu", "Advanced"],
      approval: "Policy",
      costHint: "Module-defined",
    },
  ];

  const awsOnly: CatalogItem[] =
    type === "aws"
      ? [
          {
            label: "SSO access",
            desc: "Assign Entra users to AWS roles through a governed access request.",
            icon: UserCheck,
            href: "/access",
            provider,
            via: "Entra ID",
            category: "Security & Access",
            tags: ["Access", "SAML"],
            approval: "Policy",
          },
          {
            label: "DNS zone",
            desc: "Managed Route 53 hosted zone, public or private to a VPC.",
            icon: Globe,
            href: `/dns/new?provider=${p}`,
            provider,
            via: "Route 53",
            category: "App services",
            tags: ["DNS", "Networking"],
            approval: "Auto",
            costHint: "Low baseline cost",
          },
          {
            label: "Certificate",
            desc: "ACM TLS certificate with automatic DNS validation.",
            icon: BadgeCheck,
            href: `/certs/new?provider=${p}`,
            provider,
            via: "ACM",
            category: "App services",
            tags: ["TLS", "Certificate"],
            approval: "Auto",
            costHint: "No charge for public certs",
          },
          {
            label: "Load balancer",
            desc: "Application load balancer fronting instances, IPs, or a Lambda.",
            icon: Network,
            href: `/loadbalancers/new?provider=${p}`,
            provider,
            via: "ALB",
            category: "App services",
            tags: ["Load balancer", "Networking"],
            approval: "Policy",
            costHint: "Estimate before submit",
          },
          {
            label: "API gateway",
            desc: "HTTP API routing to a Lambda or an upstream HTTP service.",
            icon: Webhook,
            href: `/apigateways/new?provider=${p}`,
            provider,
            via: "API Gateway",
            category: "App services",
            tags: ["API", "HTTP"],
            approval: "Auto",
            costHint: "Usage-based",
          },
          {
            label: "CDN",
            desc: "CloudFront distribution fronting an S3 bucket, ALB, API, or custom origin.",
            icon: Globe2,
            href: `/cdns/new?provider=${p}`,
            provider,
            via: "CloudFront",
            category: "App services",
            tags: ["CDN", "Edge"],
            approval: "Policy",
            costHint: "Usage-based",
          },
        ]
      : [];

  return [...base, ...cloudData, ...appServices, ...accessItems, ...awsOnly, ...landingZones, ...automation];
}

function ProviderPill({ provider }: { provider: Provider }) {
  const visual = providerVisual(provider.type);
  return (
    <span className="inline-flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
      {visual.label}
    </span>
  );
}

function ProviderFilterCard({
  provider,
  selected,
  onClick,
}: {
  provider: Provider;
  selected: boolean;
  onClick: () => void;
}) {
  const visual = providerVisual(provider.type);
  const Icon = visual.icon;

  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "grid min-h-[76px] min-w-[170px] grid-cols-[auto_1fr] items-center gap-3 rounded-lg border px-3 py-2 text-left transition-colors focus:outline-none focus:ring-2 focus:ring-ring/30",
        selected
          ? "border-primary/50 bg-primary/10 text-foreground shadow-sm shadow-primary/10"
          : "border-border bg-card text-muted-foreground hover:border-primary/30 hover:bg-muted/40 hover:text-foreground",
      )}
      aria-pressed={selected}
      title={`Filter catalog by ${provider.name}`}
    >
      <span className={cn("grid size-10 place-items-center rounded-lg border", visual.accent)}>
        <Icon className="size-5" />
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-semibold text-foreground">{visual.label}</span>
        <span className="mt-0.5 block truncate text-xs text-muted-foreground">{provider.name}</span>
      </span>
    </button>
  );
}

function ItemCard({ item }: { item: CatalogItem }) {
  const Icon = item.icon;
  return (
    <Card className="flex h-full flex-col p-4 transition-colors hover:border-primary/40 hover:bg-muted/40">
      <div className="flex items-start gap-3">
        <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground">
          <Icon className="size-5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-semibold text-foreground">{item.label}</h3>
            {item.via && (
              <span className="rounded-md bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-badge-primary">
                {item.via}
              </span>
            )}
          </div>
          <div className="mt-1 flex items-center gap-2">
            <ProviderPill provider={item.provider} />
            <span className="truncate text-xs text-muted-foreground">{item.provider.name}</span>
          </div>
        </div>
      </div>

      <p className="mt-3 min-h-10 text-xs leading-5 text-muted-foreground">{item.desc}</p>

      <div className="mt-3 flex flex-wrap gap-1.5">
        <Badge variant={item.approval === "Auto" ? "success" : "warning"}>{item.approval === "Auto" ? "Auto-approved" : "Approval policy"}</Badge>
        {item.ttlSupport && <Badge variant="info">TTL</Badge>}
        <Badge variant="default">Owner required</Badge>
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {item.tags.slice(0, 4).map((tag) => (
          <span key={tag} className="rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
            {tag}
          </span>
        ))}
      </div>

      <div className="mt-4 flex items-center justify-between gap-3 border-t border-border pt-3">
        <span className="text-xs text-muted-foreground">{item.costHint ?? "Cost reviewed on request"}</span>
        <Link href={item.href} className={button({ size: "sm" })}>
          Request
        </Link>
      </div>
    </Card>
  );
}

export function CatalogBrowser({ providers }: { providers: Provider[] }) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<(typeof categories)[number]>("All");
  const [providerType, setProviderType] = useState("all");
  const [selectedProviderId, setSelectedProviderId] = useState("all");

  const items = useMemo(() => providers.flatMap(itemsForProvider), [providers]);
  const providerTypes = Array.from(new Set(providers.map((p) => p.type))).sort();
  const filtered = items.filter((item) => {
    const q = query.trim().toLowerCase();
    const matchesQuery =
      q === "" ||
      [item.label, item.desc, item.provider.name, item.provider.type, item.via, ...item.tags]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(q));
    const matchesCategory = category === "All" || item.category === category;
    const matchesProviderType = providerType === "all" || item.provider.type === providerType;
    const matchesProvider = selectedProviderId === "all" || item.provider.id === selectedProviderId;
    return matchesQuery && matchesCategory && matchesProviderType && matchesProvider;
  });

  if (providers.length === 0) {
    return (
      <Card>
        <EmptyState
          icon={Server}
          title="No providers registered"
          description="Register a backend first - then approved services appear here with provider compatibility and guardrails."
        />
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <Card className="p-4">
        <div className="grid gap-3 lg:grid-cols-[1fr_auto] lg:items-center">
          <label className="relative block">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search services, providers, tags..."
              className="h-10 w-full rounded-lg border border-input bg-card pl-9 pr-3 text-sm outline-none transition focus:border-ring focus:ring-2 focus:ring-ring/20"
            />
          </label>
          <select
            value={providerType}
            onChange={(e) => {
              setProviderType(e.target.value);
              setSelectedProviderId("all");
            }}
            className="h-10 rounded-lg border border-input bg-card px-3 text-sm outline-none transition focus:border-ring focus:ring-2 focus:ring-ring/20"
          >
            <option value="all">All providers</option>
            {providerTypes.map((type) => (
              <option key={type} value={type}>
                {type.toUpperCase()}
              </option>
            ))}
          </select>
        </div>
        <div className="mt-4 flex flex-wrap gap-2">
          {categories.map((cat) => (
            <button
              key={cat}
              type="button"
              onClick={() => setCategory(cat)}
              className={cn(
                "rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors",
                category === cat
                  ? "border-primary/40 bg-primary/10 text-badge-primary"
                  : "border-border bg-card text-muted-foreground hover:bg-muted hover:text-foreground",
              )}
            >
              {cat}
            </button>
          ))}
        </div>
      </Card>

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => {
            setSelectedProviderId("all");
            setProviderType("all");
          }}
          className={cn(
            "grid min-h-[76px] min-w-[170px] grid-cols-[auto_1fr] items-center gap-3 rounded-lg border px-3 py-2 text-left transition-colors focus:outline-none focus:ring-2 focus:ring-ring/30",
            selectedProviderId === "all" && providerType === "all"
              ? "border-primary/50 bg-primary/10 text-foreground shadow-sm shadow-primary/10"
              : "border-border bg-card text-muted-foreground hover:border-primary/30 hover:bg-muted/40 hover:text-foreground",
          )}
          aria-pressed={selectedProviderId === "all" && providerType === "all"}
          title="Show catalog items from all providers"
        >
          <span className="grid size-10 place-items-center rounded-lg border border-primary/20 bg-primary/10 text-primary">
            <Layers className="size-5" />
          </span>
          <span>
            <span className="block text-sm font-semibold text-foreground">All providers</span>
            <span className="mt-0.5 block text-xs text-muted-foreground">{providers.length} connected</span>
          </span>
        </button>
        {providers.map((provider) => (
          <ProviderFilterCard
            key={provider.id}
            provider={provider}
            selected={selectedProviderId === provider.id}
            onClick={() => {
              setSelectedProviderId(provider.id);
              setProviderType("all");
            }}
          />
        ))}
      </div>

      {filtered.length === 0 ? (
        <Card>
          <EmptyState
            icon={Search}
            title="No matching services"
            description="Try a different category, provider, or search term."
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map((item) => (
            <ItemCard key={`${item.provider.id}-${item.href}-${item.label}`} item={item} />
          ))}
        </div>
      )}

      <section className="space-y-3">
        <h2 className="text-sm font-semibold tracking-tight">Composed services</h2>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          <Card className="flex flex-col p-4">
            <div className="flex items-start gap-3">
              <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground">
                <Layers className="size-5" />
              </div>
              <div>
                <h3 className="text-sm font-semibold">Environment blueprint</h3>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  Request a full environment from a governed blueprint. Component-level details are reviewed before provisioning.
                </p>
              </div>
            </div>
            <div className="mt-4 flex items-center justify-between border-t border-border pt-3">
              <span className="text-xs text-muted-foreground">Approval policy</span>
              <Link href="/environments/new" className={button({ size: "sm" })}>
                Request
              </Link>
            </div>
          </Card>
        </div>
      </section>
    </div>
  );
}
