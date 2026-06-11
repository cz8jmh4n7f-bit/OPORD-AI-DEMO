"use client";

import type { ReactNode } from "react";
import { ToastProvider } from "@/components/ui/toast";
import { ConfirmProvider } from "@/components/ui/confirm";

// AppProviders mounts the client-side UI context (toasts + confirm/prompt
// dialogs) around the whole app shell, so any client component can call
// useToast() / useConfirm().
export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ToastProvider>
      <ConfirmProvider>{children}</ConfirmProvider>
    </ToastProvider>
  );
}
