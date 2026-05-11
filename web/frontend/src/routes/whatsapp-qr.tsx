import { createFileRoute } from "@tanstack/react-router"

import { WhatsAppQRPage } from "@/components/channels/whatsapp-qr-page"

export const Route = createFileRoute("/whatsapp-qr")({
  component: WhatsAppQRPage,
})
