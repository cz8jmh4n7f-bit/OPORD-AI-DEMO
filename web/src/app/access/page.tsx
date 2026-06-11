import { PageHeader } from "@/components/page-header";
import { EntraGrantForm } from "@/components/entra-grant-form";

export const metadata = { title: "SSO access" };

export default function AccessPage() {
  return (
    <div className="max-w-3xl space-y-6">
      <PageHeader
        title="AWS SSO access (SAML / Entra)"
        description="Grant Entra (Azure AD) users access to an AWS account via its SAML enterprise app: OPORD ensures the app role that carries the AWS Role claim, optionally invites B2B guests, then assigns each user. This is access TO AWS (not to OPORD). Requires the Microsoft Graph integration (AZURE_* creds in Vault at opord/azure/graph)."
      />
      <EntraGrantForm />
    </div>
  );
}
