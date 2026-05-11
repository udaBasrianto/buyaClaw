import { launcherFetch } from "@/api/http"

export interface WhatsAppQRState {
  status: "waiting" | "scanned" | "connected" | "expired" | "error"
  qr_code?: string
  qr_data_uri?: string
  phone?: string
  error?: string
  updated_at: string
}

export async function getWhatsAppQRStatus(): Promise<WhatsAppQRState> {
  const res = await launcherFetch("/api/whatsapp/qr")
  if (!res.ok) throw new Error(`WhatsApp API error: ${res.status}`)
  return res.json()
}
