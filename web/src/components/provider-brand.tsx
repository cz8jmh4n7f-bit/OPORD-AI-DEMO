// ProviderBrand renders compact provider marks for catalog/provider controls.
// Keep these small and recognizable; provider cards still carry the instance
// name separately, so the brand mark does not need to repeat every detail.
//
// Pass `iconOnly` to render just the logo mark (no wordmark) - for tight spots
// like the FinOps cloud tiles, where the provider name is shown separately and a
// wordmark would overflow.

type Props = {
  type: string;
  className?: string;
  iconOnly?: boolean;
};

export function ProviderBrand({ type, className, iconOnly }: Props) {
  if (type === "aws") {
    return (
      <svg
        viewBox="0 0 24 24"
        className={`h-6 w-auto text-[#FF9900] ${className ?? ""}`}
        fill="currentColor"
        role="img"
        aria-label="Amazon Web Services"
      >
        <title>Amazon Web Services</title>
        <path d="M6.763 10.036c0 .296.032.535.088.71.064.176.144.368.256.576.04.063.056.127.056.183 0 .08-.048.16-.152.24l-.503.335a.383.383 0 0 1-.208.072c-.08 0-.16-.04-.24-.112a2.47 2.47 0 0 1-.287-.375 6.18 6.18 0 0 1-.248-.471c-.622.734-1.405 1.101-2.347 1.101-.67 0-1.205-.191-1.596-.574-.391-.384-.59-.894-.59-1.533 0-.678.239-1.23.726-1.644.487-.415 1.133-.623 1.955-.623.272 0 .551.024.846.064.296.04.6.104.918.176v-.583c0-.607-.127-1.03-.375-1.277-.255-.248-.686-.367-1.3-.367-.28 0-.568.031-.863.103-.295.072-.583.16-.862.272a2.287 2.287 0 0 1-.28.104.488.488 0 0 1-.127.023c-.112 0-.168-.08-.168-.247v-.391c0-.128.016-.224.056-.28a.597.597 0 0 1 .224-.167c.279-.144.614-.264 1.005-.36a4.84 4.84 0 0 1 1.246-.151c.95 0 1.644.216 2.091.647.439.43.662 1.085.662 1.963v2.586zm-3.24 1.214c.263 0 .534-.048.822-.144.287-.096.543-.271.758-.51.128-.152.224-.32.272-.512.047-.191.08-.423.08-.694v-.335a6.66 6.66 0 0 0-.735-.136 6.02 6.02 0 0 0-.75-.048c-.535 0-.926.104-1.19.32-.263.215-.39.518-.39.917 0 .376.095.655.295.846.191.2.47.296.838.296zm6.41.862c-.144 0-.24-.024-.304-.08-.064-.048-.12-.16-.168-.311L7.586 5.55a1.398 1.398 0 0 1-.072-.32c0-.128.064-.2.191-.2h.783c.151 0 .255.025.31.08.065.048.113.16.16.312l1.342 5.284 1.245-5.284c.04-.16.088-.264.151-.312a.549.549 0 0 1 .32-.08h.638c.152 0 .256.025.32.08.063.048.12.16.151.312l1.261 5.348 1.381-5.348c.048-.16.104-.264.16-.312a.52.52 0 0 1 .311-.08h.743c.127 0 .2.065.2.2 0 .04-.009.08-.017.128a1.137 1.137 0 0 1-.056.2l-1.923 6.17c-.048.16-.104.263-.168.311a.51.51 0 0 1-.303.08h-.687c-.151 0-.255-.024-.32-.08-.063-.056-.119-.16-.15-.32l-1.238-5.148-1.23 5.14c-.04.16-.087.264-.15.32-.065.056-.177.08-.32.08zm10.256.215c-.415 0-.83-.048-1.229-.143-.399-.096-.71-.2-.918-.32-.128-.071-.215-.151-.247-.223a.563.563 0 0 1-.048-.224v-.407c0-.167.064-.247.183-.247.048 0 .096.008.144.024.048.016.12.048.2.08.272.12.566.215.878.279.319.064.63.096.95.096.503 0 .894-.088 1.165-.264a.86.86 0 0 0 .415-.758.777.777 0 0 0-.215-.559c-.144-.151-.416-.287-.807-.415l-1.157-.36c-.583-.183-1.014-.454-1.277-.813a1.902 1.902 0 0 1-.4-1.158c0-.335.073-.63.216-.886.144-.255.335-.479.575-.654.24-.184.51-.32.83-.415.32-.096.655-.136 1.006-.136.175 0 .359.008.535.032.183.024.35.056.518.088.16.04.312.08.455.127.144.048.256.096.336.144a.69.69 0 0 1 .24.2.43.43 0 0 1 .071.263v.375c0 .168-.064.256-.184.256a.83.83 0 0 1-.303-.096 3.652 3.652 0 0 0-1.532-.311c-.455 0-.815.071-1.062.223-.248.152-.375.383-.375.71 0 .224.08.416.24.567.16.152.454.304.877.44l1.134.358c.574.184.99.44 1.237.767.247.327.367.702.367 1.117 0 .343-.072.655-.207.926-.144.272-.336.511-.583.703-.248.2-.543.343-.886.447-.36.111-.734.167-1.142.167zM21.698 16.207c-2.626 1.94-6.442 2.969-9.722 2.969-4.598 0-8.74-1.7-11.87-4.526-.247-.224-.024-.527.27-.351 3.384 1.963 7.559 3.153 11.877 3.153 2.914 0 6.114-.607 9.06-1.852.439-.2.814.287.384.607zM22.792 14.961c-.336-.43-2.22-.207-3.074-.103-.255.032-.295-.192-.063-.36 1.5-1.053 3.967-.75 4.254-.399.287.36-.08 2.826-1.485 4.007-.215.184-.423.088-.327-.151.32-.79 1.03-2.57.695-2.994z" />
      </svg>
    );
  }
  if (type === "azure") {
    return (
      <svg
        viewBox="0 0 24 24"
        className={`h-6 w-auto text-[#0078D4] ${className ?? ""}`}
        fill="currentColor"
        role="img"
        aria-label="Microsoft Azure"
      >
        <title>Microsoft Azure</title>
        <path d="M13.05 4.24 6.56 18.05H2.13L8.5 7.6h4.55zm.59 1.81 9.41 15.7H8.59l5.9-9.49 3.05 5.13H22z" />
      </svg>
    );
  }
  if (type === "gcp") {
    const icon = (
      <svg viewBox="0 0 32 24" className={`h-6 w-auto ${iconOnly ? (className ?? "") : ""}`} aria-hidden="true">
        <path d="M20.6 7.2a7 7 0 0 0-13 2.3A5.8 5.8 0 0 0 6 21h13.6a6.4 6.4 0 0 0 1-12.8z" fill="none" stroke="#4285F4" strokeLinecap="round" strokeLinejoin="round" strokeWidth="3.2" />
        <path d="M7.6 9.5a7 7 0 0 1 13-2.3" fill="none" stroke="#34A853" strokeLinecap="round" strokeWidth="3.2" />
        <path d="M20.6 7.2a6.4 6.4 0 0 1 5.3 6.3" fill="none" stroke="#FBBC04" strokeLinecap="round" strokeWidth="3.2" />
        <path d="M19.6 21H6a5.8 5.8 0 0 1 1.6-11.5" fill="none" stroke="#EA4335" strokeLinecap="round" strokeWidth="3.2" />
      </svg>
    );
    if (iconOnly) {
      return (
        <span className="inline-flex" aria-label="Google Cloud">
          {icon}
        </span>
      );
    }
    return (
      <span className={`inline-flex items-center gap-1.5 ${className ?? ""}`} aria-label="Google Cloud">
        {icon}
        <span className="text-sm font-semibold tracking-tight text-[#4285F4]">Google Cloud</span>
      </span>
    );
  }
  if (type === "proxmox") {
    const badge = (
      <span className="grid size-5 place-items-center rounded bg-[#E57000] text-[11px] font-black leading-none text-white">
        P
      </span>
    );
    if (iconOnly) {
      return (
        <span className="inline-flex" aria-label="Proxmox">
          {badge}
        </span>
      );
    }
    return (
      <span className={`inline-flex items-center gap-1.5 ${className ?? ""}`} aria-label="Proxmox">
        {badge}
        <span className="text-sm font-bold tracking-tight text-[#E57000]">Proxmox</span>
      </span>
    );
  }
  if (type === "vsphere") {
    if (iconOnly) {
      return (
        <span
          className="grid size-5 place-items-center rounded bg-[#607078] text-[10px] font-bold leading-none text-white"
          aria-label="VMware vSphere"
        >
          vS
        </span>
      );
    }
    return (
      <span className={`inline-flex items-baseline gap-1.5 ${className ?? ""}`} aria-label="VMware vSphere">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-[#607078] dark:text-[#9DA5AC]">
          VMware
        </span>
        <span className="text-sm font-bold tracking-tight text-[#607078] dark:text-[#C3CBD1]">vSphere</span>
      </span>
    );
  }
  return (
    <span className={`text-sm font-medium text-muted-foreground ${className ?? ""}`}>
      {type}
    </span>
  );
}
