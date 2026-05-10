import { IconLoader2, IconRefresh, IconSearch } from "@tabler/icons-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  type ModelDiscoveryItem,
  type ModelProviderOption,
  addModel,
  fetchAvailableModels,
  setDefaultModel,
} from "@/api/models"
import { ConfigChangeNotice } from "@/components/config-change-notice"
import { maskedSecretPlaceholder } from "@/components/secret-placeholder"
import {
  AdvancedSection,
  Field,
  KeyInput,
  SwitchCardField,
} from "@/components/shared-form"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import { showSaveSuccessOrRestartToast } from "@/lib/restart-required"
import { refreshGatewayState } from "@/store/gateway"

import {
  findProviderOption,
  getProviderDefaultAPIBase,
  getProviderDefaultAuthMethod,
  getProviderLabel,
  getSortedProviderOptions,
  isProviderAuthMethodLocked,
} from "./provider-label"

interface AddForm {
  modelName: string
  provider: string
  model: string
  apiBase: string
  apiKey: string
  proxy: string
  authMethod: string
  connectMode: string
  workspace: string
  rpm: string
  maxTokensField: string
  requestTimeout: string
  thinkingLevel: string
  toolSchemaTransform: string
  extraBody: string
  customHeaders: string
}

const EMPTY_ADD_FORM: AddForm = {
  modelName: "",
  provider: "openai",
  model: "",
  apiBase: "",
  apiKey: "",
  proxy: "",
  authMethod: "",
  connectMode: "",
  workspace: "",
  rpm: "",
  maxTokensField: "",
  requestTimeout: "",
  thinkingLevel: "",
  toolSchemaTransform: "",
  extraBody: "",
  customHeaders: "",
}

interface AddModelSheetProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  existingModelNames: string[]
  providerOptions: ModelProviderOption[]
}

export function AddModelSheet({
  open,
  onClose,
  onSaved,
  existingModelNames,
  providerOptions,
}: AddModelSheetProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<AddForm>(EMPTY_ADD_FORM)
  const [saving, setSaving] = useState(false)
  const [setAsDefault, setSetAsDefault] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<
    Partial<Record<keyof AddForm, string>>
  >({})
  const [serverError, setServerError] = useState("")

  // Auto-fetch model list state
  const [availableModels, setAvailableModels] = useState<ModelDiscoveryItem[]>([])
  const [fetchingModels, setFetchingModels] = useState(false)
  const [modelFetchError, setModelFetchError] = useState<string | null>(null)
  const [showModelTable, setShowModelTable] = useState(false)
  const [modelSearch, setModelSearch] = useState("")
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const apiKeyPlaceholder = maskedSecretPlaceholder(
    form.apiKey,
    t("models.field.apiKeyPlaceholder"),
  )
  const sortedProviderOptions = useMemo(
    () => getSortedProviderOptions(providerOptions),
    [providerOptions],
  )
  const creatableProviderOptions = useMemo(
    () => sortedProviderOptions.filter((option) => option.create_allowed),
    [sortedProviderOptions],
  )
  const selectedProviderOption = findProviderOption(
    form.provider,
    providerOptions,
  )
  const authMethodLocked = isProviderAuthMethodLocked(
    form.provider,
    providerOptions,
  )
  const defaultAuthMethod = getProviderDefaultAuthMethod(
    form.provider,
    providerOptions,
  )
  const effectiveAuthMethod = (
    authMethodLocked ? defaultAuthMethod : form.authMethod
  )
    .trim()
    .toLowerCase()
  const isOAuth = effectiveAuthMethod === "oauth"
  const defaultModelAllowed =
    selectedProviderOption?.default_model_allowed !== false
  const apiBasePlaceholder =
    getProviderDefaultAPIBase(form.provider, providerOptions) ||
    "https://api.example.com/v1"
  const isDirty =
    JSON.stringify(form) !== JSON.stringify(EMPTY_ADD_FORM) || setAsDefault

  useEffect(() => {
    if (open) {
      setForm(EMPTY_ADD_FORM)
      setSetAsDefault(false)
      setFieldErrors({})
      setServerError("")
      setAvailableModels([])
      setModelFetchError(null)
      setFetchingModels(false)
      setShowModelTable(false)
      setModelSearch("")
    }
  }, [open])

  // Debounced auto-fetch when apiBase or apiKey changes
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)

    const apiBase = form.apiBase.trim()
    const apiKey = form.apiKey.trim()

    // Need at least apiBase to fetch (some providers allow empty key)
    if (!apiBase) {
      setAvailableModels([])
      setModelFetchError(null)
      return
    }

    debounceRef.current = setTimeout(async () => {
      setFetchingModels(true)
      setModelFetchError(null)
      try {
        const result = await fetchAvailableModels(form.provider, apiBase, apiKey)
        if (result.error) {
          setModelFetchError(result.error)
          setAvailableModels([])
        } else {
          setAvailableModels(result.models ?? [])
          setModelFetchError(null)
          if ((result.models ?? []).length > 0) {
            setShowModelTable(true)
          }
        }
      } catch {
        setModelFetchError("Failed to fetch models")
        setAvailableModels([])
      } finally {
        setFetchingModels(false)
      }
    }, 600)

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [form.apiBase, form.apiKey, form.provider])

  const validate = (): boolean => {
    const errors: Partial<Record<keyof AddForm, string>> = {}
    const modelName = form.modelName.trim()
    if (!modelName) {
      errors.modelName = t("models.add.errorRequired")
    } else if (existingModelNames.some((name) => name.trim() === modelName)) {
      errors.modelName = t("models.add.errorDuplicateModelName")
    }
    if (!selectedProviderOption) {
      errors.provider = t("models.field.providerInvalid")
    }
    if (!form.model.trim()) errors.model = t("models.add.errorRequired")
    setFieldErrors(errors)
    return Object.keys(errors).length === 0
  }

  const setField =
    (key: keyof AddForm) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setForm((f) => ({ ...f, [key]: e.target.value }))
      if (fieldErrors[key]) {
        setFieldErrors((prev) => ({ ...prev, [key]: undefined }))
      }
    }

  const setProvider = (value: string) => {
    setForm((f) => {
      const previousOption = findProviderOption(f.provider, providerOptions)
      const nextOption = findProviderOption(value, providerOptions)
      let authMethod = f.authMethod
      if (nextOption?.auth_method_locked) {
        authMethod = nextOption.default_auth_method ?? ""
      } else if (
        previousOption?.auth_method_locked &&
        f.authMethod === (previousOption.default_auth_method ?? "")
      ) {
        authMethod = ""
      }
      return { ...f, provider: value, authMethod }
    })
    const nextOption = findProviderOption(value, providerOptions)
    if (nextOption?.default_model_allowed === false) {
      setSetAsDefault(false)
    }
    if (fieldErrors.provider) {
      setFieldErrors((prev) => ({ ...prev, provider: undefined }))
    }
  }

  const handleSave = async () => {
    if (!validate()) return
    setSaving(true)
    setServerError("")
    try {
      const modelName = form.modelName.trim()
      const modelId = form.model.trim()
      await addModel({
        model_name: modelName,
        provider: form.provider.trim(),
        model: modelId,
        api_base: form.apiBase.trim() || undefined,
        api_key: form.apiKey.trim() || undefined,
        proxy: form.proxy.trim() || undefined,
        auth_method: authMethodLocked
          ? defaultAuthMethod || undefined
          : form.authMethod.trim() || undefined,
        connect_mode: form.connectMode.trim() || undefined,
        workspace: form.workspace.trim() || undefined,
        rpm: form.rpm ? Number(form.rpm) : undefined,
        max_tokens_field: form.maxTokensField.trim() || undefined,
        request_timeout: form.requestTimeout
          ? Number(form.requestTimeout)
          : undefined,
        thinking_level: form.thinkingLevel.trim() || undefined,
        tool_schema_transform: form.toolSchemaTransform.trim() || undefined,
        extra_body: form.extraBody.trim()
          ? JSON.parse(form.extraBody.trim())
          : undefined,
        custom_headers: form.customHeaders.trim()
          ? JSON.parse(form.customHeaders.trim())
          : undefined,
      })
      if (setAsDefault) {
        await setDefaultModel(modelName)
      }
      const gateway = await refreshGatewayState({ force: true })
      showSaveSuccessOrRestartToast(
        t,
        t("models.add.saveSuccess"),
        modelName,
        gateway?.restartRequired === true,
      )
      onSaved()
      onClose()
    } catch (e) {
      setServerError(e instanceof Error ? e.message : t("models.add.saveError"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      <SheetContent
        side="right"
        className="flex flex-col gap-0 p-0 data-[side=right]:!w-full data-[side=right]:sm:!w-[560px] data-[side=right]:sm:!max-w-[560px]"
      >
        <SheetHeader className="border-b-muted border-b px-6 py-5">
          <SheetTitle className="text-base">{t("models.add.title")}</SheetTitle>
          <SheetDescription className="text-xs">
            {t("models.add.description")}
          </SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="space-y-5 px-6 py-5">
            <Field
              label={t("models.add.modelName")}
              hint={t("models.add.modelNameHint")}
            >
              <Input
                value={form.modelName}
                onChange={setField("modelName")}
                placeholder={t("models.add.modelNamePlaceholder")}
                aria-invalid={!!fieldErrors.modelName}
              />
              {fieldErrors.modelName && (
                <p className="text-destructive text-xs">
                  {fieldErrors.modelName}
                </p>
              )}
            </Field>

            <Field
              label={t("models.field.provider")}
              hint={t("models.field.providerHint")}
              error={fieldErrors.provider}
              required
            >
              <Select
                value={selectedProviderOption?.id}
                onValueChange={setProvider}
              >
                <SelectTrigger
                  className="w-full"
                  aria-invalid={!!fieldErrors.provider}
                >
                  <SelectValue
                    placeholder={t("models.field.providerPlaceholder")}
                  />
                </SelectTrigger>
                <SelectContent>
                  {creatableProviderOptions.map((option) => (
                    <SelectItem key={option.id} value={option.id}>
                      {getProviderLabel(option.id)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>

            <Field
              label={t("models.add.modelId")}
              hint={t("models.add.modelIdHint")}
            >
              <div className="space-y-2">
                {/* Input + refresh button */}
                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <Input
                      value={form.model}
                      onChange={(e) => {
                        setField("model")(e)
                        setModelSearch(e.target.value)
                      }}
                      placeholder={t("models.add.modelIdPlaceholder")}
                      className="font-mono text-sm"
                      aria-invalid={!!fieldErrors.model}
                      autoComplete="off"
                    />
                    {fetchingModels && (
                      <IconLoader2 className="text-muted-foreground absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin" />
                    )}
                  </div>
                  {availableModels.length > 0 && (
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      title="Refresh model list"
                      onClick={async () => {
                        setFetchingModels(true)
                        setModelFetchError(null)
                        try {
                          const result = await fetchAvailableModels(
                            form.provider,
                            form.apiBase.trim(),
                            form.apiKey.trim(),
                          )
                          setAvailableModels(result.models ?? [])
                          if (result.error) setModelFetchError(result.error)
                        } finally {
                          setFetchingModels(false)
                        }
                      }}
                    >
                      <IconRefresh className="h-4 w-4" />
                    </Button>
                  )}
                </div>

                {/* Status line */}
                {!fetchingModels && availableModels.length > 0 && !modelFetchError && (
                  <div className="flex items-center justify-between">
                    <p className="text-muted-foreground text-xs">
                      {availableModels.length} models available
                    </p>
                    <button
                      type="button"
                      className="text-primary text-xs underline-offset-2 hover:underline"
                      onClick={() => setShowModelTable((v) => !v)}
                    >
                      {showModelTable ? "Hide list" : "Browse & select"}
                    </button>
                  </div>
                )}
                {!fetchingModels && modelFetchError && (
                  <p className="text-muted-foreground text-xs">
                    ⚠ {modelFetchError} — type the model ID manually
                  </p>
                )}

                {/* Model table */}
                {showModelTable && availableModels.length > 0 && (
                  <ModelTable
                    models={availableModels}
                    search={modelSearch}
                    onSearch={setModelSearch}
                    onSelect={(id) => {
                      setForm((f) => ({ ...f, model: id }))
                      setShowModelTable(false)
                      if (fieldErrors.model) {
                        setFieldErrors((prev) => ({ ...prev, model: undefined }))
                      }
                    }}
                  />
                )}
              </div>

              {fieldErrors.model && (
                <p className="text-destructive text-xs">{fieldErrors.model}</p>
              )}
            </Field>

            {!isOAuth && (
              <Field label={t("models.field.apiKey")}>
                <KeyInput
                  value={form.apiKey}
                  onChange={(v) => setForm((f) => ({ ...f, apiKey: v }))}
                  placeholder={apiKeyPlaceholder}
                />
              </Field>
            )}

            <Field
              label={t("models.field.apiBase")}
              hint={isOAuth ? t("models.edit.oauthNote") : undefined}
            >
              <Input
                value={form.apiBase}
                onChange={setField("apiBase")}
                placeholder={apiBasePlaceholder}
                disabled={isOAuth}
              />
            </Field>

            <SwitchCardField
              label={t("models.defaultOnSave.label")}
              hint={
                defaultModelAllowed
                  ? t("models.defaultOnSave.description")
                  : t("models.defaultOnSave.unsupportedProvider")
              }
              checked={setAsDefault}
              onCheckedChange={setSetAsDefault}
              disabled={!defaultModelAllowed}
            />

            <AdvancedSection>
              <Field
                label={t("models.field.proxy")}
                hint={t("models.field.proxyHint")}
              >
                <Input
                  value={form.proxy}
                  onChange={setField("proxy")}
                  placeholder="http://127.0.0.1:7890"
                />
              </Field>

              <Field
                label={t("models.field.authMethod")}
                hint={
                  authMethodLocked
                    ? t("models.field.authMethodManagedHint")
                    : t("models.field.authMethodHint")
                }
              >
                <Input
                  value={authMethodLocked ? defaultAuthMethod : form.authMethod}
                  onChange={setField("authMethod")}
                  placeholder="oauth"
                  disabled={authMethodLocked}
                />
              </Field>

              <Field
                label={t("models.field.connectMode")}
                hint={t("models.field.connectModeHint")}
              >
                <Input
                  value={form.connectMode}
                  onChange={setField("connectMode")}
                  placeholder="stdio"
                />
              </Field>

              <Field
                label={t("models.field.workspace")}
                hint={t("models.field.workspaceHint")}
              >
                <Input
                  value={form.workspace}
                  onChange={setField("workspace")}
                  placeholder="/path/to/workspace"
                />
              </Field>

              <Field
                label={t("models.field.requestTimeout")}
                hint={t("models.field.requestTimeoutHint")}
              >
                <Input
                  value={form.requestTimeout}
                  onChange={setField("requestTimeout")}
                  placeholder="60"
                  type="number"
                  min={0}
                />
              </Field>

              <Field
                label={t("models.field.rpm")}
                hint={t("models.field.rpmHint")}
              >
                <Input
                  value={form.rpm}
                  onChange={setField("rpm")}
                  placeholder="60"
                  type="number"
                  min={0}
                />
              </Field>

              <Field
                label={t("models.field.thinkingLevel")}
                hint={t("models.field.thinkingLevelHint")}
              >
                <Input
                  value={form.thinkingLevel}
                  onChange={setField("thinkingLevel")}
                  placeholder="off"
                />
              </Field>

              <Field
                label={t("models.field.maxTokensField")}
                hint={t("models.field.maxTokensFieldHint")}
              >
                <Input
                  value={form.maxTokensField}
                  onChange={setField("maxTokensField")}
                  placeholder="max_completion_tokens"
                />
              </Field>

              <Field
                label={t("models.field.toolSchemaTransform")}
                hint={t("models.field.toolSchemaTransformHint")}
              >
                <Input
                  value={form.toolSchemaTransform}
                  onChange={setField("toolSchemaTransform")}
                  placeholder="google"
                />
              </Field>

              <Field
                label={t("models.field.extraBody")}
                hint={t("models.field.extraBodyHint")}
              >
                <Textarea
                  value={form.extraBody}
                  onChange={setField("extraBody")}
                  placeholder='{"key": "value"}'
                  rows={3}
                />
              </Field>

              <Field
                label={t("models.field.customHeaders")}
                hint={t("models.field.customHeadersHint")}
              >
                <Textarea
                  value={form.customHeaders}
                  onChange={setField("customHeaders")}
                  placeholder='{"X-Source": "coding-plan"}'
                  rows={3}
                />
              </Field>
            </AdvancedSection>

            {serverError && (
              <p className="text-destructive bg-destructive/10 rounded-md px-3 py-2 text-sm">
                {serverError}
              </p>
            )}
          </div>
        </div>

        <SheetFooter className="border-t-muted border-t px-6 py-4">
          {isDirty && (
            <ConfigChangeNotice
              kind="save"
              title={t("common.saveChangesTitle")}
              description={t("models.unsavedPrompt")}
            />
          )}
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={!isDirty || saving}>
            {saving && <IconLoader2 className="size-4 animate-spin" />}
            {t("models.add.confirm")}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

// ---------------------------------------------------------------------------
// ModelTable — searchable table for browsing and selecting a model
// ---------------------------------------------------------------------------

interface ModelTableProps {
  models: ModelDiscoveryItem[]
  search: string
  onSearch: (v: string) => void
  onSelect: (id: string) => void
}

function ModelTable({ models, search, onSearch, onSelect }: ModelTableProps) {
  const filtered = models.filter((m) => {
    if (!search) return true
    const q = search.toLowerCase()
    return (
      m.id.toLowerCase().includes(q) ||
      (m.name ?? "").toLowerCase().includes(q) ||
      (m.provider ?? "").toLowerCase().includes(q)
    )
  })

  return (
    <div className="border-border rounded-md border">
      {/* Search bar */}
      <div className="border-border flex items-center gap-2 border-b px-3 py-2">
        <IconSearch className="text-muted-foreground h-4 w-4 shrink-0" />
        <input
          type="text"
          className="bg-transparent w-full text-sm outline-none placeholder:text-muted-foreground"
          placeholder="Search models..."
          value={search}
          onChange={(e) => onSearch(e.target.value)}
          autoFocus
        />
        {search && (
          <button
            type="button"
            className="text-muted-foreground hover:text-foreground text-xs"
            onClick={() => onSearch("")}
          >
            ✕
          </button>
        )}
      </div>

      {/* Table header */}
      <div className="border-border bg-muted/40 grid grid-cols-[1fr_auto] border-b px-3 py-1.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        <span>MODEL</span>
        <span className="text-right">PROVIDER</span>
      </div>

      {/* Rows */}
      <div className="max-h-72 overflow-y-auto">
        {filtered.length === 0 ? (
          <div className="text-muted-foreground px-3 py-6 text-center text-sm">
            No models match "{search}"
          </div>
        ) : (
          filtered.map((m) => (
            <button
              key={m.id}
              type="button"
              className="hover:bg-accent/60 grid w-full grid-cols-[1fr_auto] items-center gap-3 border-b border-border/40 px-3 py-3 text-left last:border-0"
              onClick={() => onSelect(m.id)}
            >
              {/* Left: model name + badges */}
              <div className="flex min-w-0 flex-col gap-0.5">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="truncate font-medium text-sm">
                    {m.name || m.id}
                  </span>
                  {m.is_free && (
                    <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/40 dark:text-green-400">
                      Free
                    </span>
                  )}
                  {!m.is_free && m.price_prompt && m.price_prompt !== "Free" && (
                    <span className="rounded-full bg-blue-50 px-2 py-0.5 text-xs text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">
                      {m.price_prompt} / 1M in
                    </span>
                  )}
                </div>
                {m.id !== m.name && m.name && (
                  <span className="text-muted-foreground truncate font-mono text-xs">
                    {m.id}
                  </span>
                )}
                {m.context_length && m.context_length > 0 && (
                  <span className="text-muted-foreground text-xs">
                    {(m.context_length / 1000).toFixed(0)}K ctx
                  </span>
                )}
              </div>

              {/* Right: provider badge */}
              {m.provider && (
                <span className="shrink-0 rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">
                  {m.provider}
                </span>
              )}
            </button>
          ))
        )}
      </div>

      {filtered.length > 0 && (
        <div className="text-muted-foreground border-t border-border/40 px-3 py-1.5 text-xs">
          {filtered.length} of {models.length} models
        </div>
      )}
    </div>
  )
}
