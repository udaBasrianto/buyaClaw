import {
  IconBrandWhatsapp,
  IconCheck,
  IconLoader2,
  IconRefresh,
  IconAlertCircle,
  IconClock,
} from "@tabler/icons-react"
import { useQuery } from "@tanstack/react-query"

import { getWhatsAppQRStatus, type WhatsAppQRState } from "@/api/whatsapp"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function WhatsAppQRPage() {
  const { data, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: ["whatsapp-qr"],
    queryFn: getWhatsAppQRStatus,
    refetchInterval: (query) => {
      const status = query.state.data?.status
      // Poll faster when waiting for scan, slower when connected
      if (status === "waiting" || status === "scanned") return 2000
      if (status === "connected") return 10000
      return 5000
    },
  })

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader title="WhatsApp">
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetch()}
          disabled={isFetching}
          className="gap-2"
        >
          <IconRefresh className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </PageHeader>

      <Card className="mx-auto w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="flex items-center justify-center gap-2">
            <IconBrandWhatsapp className="h-6 w-6 text-green-500" />
            WhatsApp Connection
          </CardTitle>
          <CardDescription>
            Link your WhatsApp account to enable AI chat via WhatsApp.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-4">
          {isLoading && (
            <div className="flex flex-col items-center gap-3 py-8">
              <IconLoader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              <p className="text-muted-foreground text-sm">Loading status...</p>
            </div>
          )}

          {error && (
            <div className="flex flex-col items-center gap-3 py-8">
              <IconAlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">
                Failed to load WhatsApp status.
              </p>
              <p className="text-muted-foreground text-xs">
                Make sure WhatsApp is enabled in config with <code>use_native: true</code>
              </p>
            </div>
          )}

          {data && !isLoading && <QRStatusView state={data} onRefresh={refetch} />}
        </CardContent>
      </Card>

      {/* Instructions */}
      <Card className="mx-auto w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-sm font-medium">How to connect</CardTitle>
        </CardHeader>
        <CardContent className="text-muted-foreground space-y-2 text-sm">
          <p>1. Enable WhatsApp in <strong>Config</strong> with <code>use_native: true</code></p>
          <p>2. Restart the gateway</p>
          <p>3. A QR code will appear above</p>
          <p>4. Open WhatsApp on your phone → <strong>Linked Devices</strong> → <strong>Link a Device</strong></p>
          <p>5. Scan the QR code</p>
          <p>6. Done! Send a message to your number to chat with AI</p>
        </CardContent>
      </Card>
    </div>
  )
}

function QRStatusView({ state, onRefresh }: { state: WhatsAppQRState; onRefresh: () => void }) {
  switch (state.status) {
    case "connected":
      return (
        <div className="flex flex-col items-center gap-3 py-6">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
            <IconCheck className="h-8 w-8 text-green-600 dark:text-green-400" />
          </div>
          <p className="text-lg font-medium text-green-700 dark:text-green-400">Connected</p>
          {state.phone && (
            <Badge variant="secondary" className="text-sm">
              +{state.phone}
            </Badge>
          )}
          <p className="text-muted-foreground text-xs">
            WhatsApp is linked and ready to receive messages.
          </p>
        </div>
      )

    case "waiting":
      return (
        <div className="flex flex-col items-center gap-4 py-4">
          {state.qr_data_uri ? (
            <>
              <div className="rounded-lg border bg-white p-3">
                <img
                  src={state.qr_data_uri}
                  alt="WhatsApp QR Code"
                  className="h-64 w-64"
                  style={{ imageRendering: "pixelated" }}
                />
              </div>
              <p className="text-muted-foreground text-sm">
                Scan this QR code with WhatsApp
              </p>
              <Badge variant="outline" className="gap-1">
                <IconClock className="h-3 w-3" />
                Waiting for scan...
              </Badge>
            </>
          ) : (
            <div className="flex flex-col items-center gap-3 py-8">
              <IconLoader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              <p className="text-muted-foreground text-sm">
                Waiting for QR code from gateway...
              </p>
              <p className="text-muted-foreground text-xs">
                Make sure WhatsApp channel is enabled and gateway is running.
              </p>
            </div>
          )}
        </div>
      )

    case "scanned":
      return (
        <div className="flex flex-col items-center gap-3 py-6">
          <IconLoader2 className="h-8 w-8 animate-spin text-green-500" />
          <p className="font-medium">QR Scanned!</p>
          <p className="text-muted-foreground text-sm">Confirming connection...</p>
        </div>
      )

    case "expired":
      return (
        <div className="flex flex-col items-center gap-3 py-6">
          <IconClock className="h-8 w-8 text-yellow-500" />
          <p className="font-medium text-yellow-700 dark:text-yellow-400">QR Code Expired</p>
          <p className="text-muted-foreground text-sm">
            The QR code has expired. Restart the gateway to get a new one.
          </p>
          <Button variant="outline" size="sm" onClick={onRefresh} className="gap-2">
            <IconRefresh className="h-4 w-4" />
            Check Again
          </Button>
        </div>
      )

    case "error":
      return (
        <div className="flex flex-col items-center gap-3 py-6">
          <IconAlertCircle className="h-8 w-8 text-destructive" />
          <p className="font-medium text-destructive">Error</p>
          <p className="text-muted-foreground text-sm">{state.error || "Unknown error"}</p>
          <Button variant="outline" size="sm" onClick={onRefresh} className="gap-2">
            <IconRefresh className="h-4 w-4" />
            Retry
          </Button>
        </div>
      )

    default:
      return null
  }
}
