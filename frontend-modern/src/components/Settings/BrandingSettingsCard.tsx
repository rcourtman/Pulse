import { Show, createSignal, type Accessor, type Component, type Setter } from 'solid-js';
import ImageIcon from 'lucide-solid/icons/image';
import Trash2 from 'lucide-solid/icons/trash-2';
import { Button } from '@/components/shared/Button';
import { FeatureGateSection } from '@/components/shared/FeatureGateSection';
import { formControl, formHelpText, formLabel } from '@/components/shared/Form';
import { hasFeature } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import type { ReportBrandSettings } from '@/types/config';

export const BRAND_LOGO_MAX_BYTES = 36 * 1024;

type BrandLogoFormat = NonNullable<ReportBrandSettings['logoFormat']>;

export interface BrandingSettingsCardProps {
  displayName: Accessor<string>;
  setDisplayName: Setter<string>;
  logoBase64: Accessor<string>;
  setLogoBase64: Setter<string>;
  logoFormat: Accessor<BrandLogoFormat>;
  setLogoFormat: Setter<BrandLogoFormat>;
  setHasUnsavedChanges: Setter<boolean>;
}

export function brandLogoFormatForFile(file: Pick<File, 'type' | 'name'>): BrandLogoFormat | null {
  const mime = file.type.toLowerCase();
  if (mime === 'image/png') return 'png';
  if (mime === 'image/jpeg') return 'jpg';
  if (mime === 'image/gif') return 'gif';

  const extension = file.name.toLowerCase().split('.').pop();
  if (extension === 'png') return 'png';
  if (extension === 'jpg' || extension === 'jpeg') return 'jpg';
  if (extension === 'gif') return 'gif';
  return null;
}

export function brandingLogoPreview(logoBase64: string, logoFormat: BrandLogoFormat): string {
  const value = logoBase64.trim();
  if (!value) return '';
  if (value.startsWith('data:image/')) return value;
  if (!logoFormat) return '';
  const mime = logoFormat === 'jpg' || logoFormat === 'jpeg' ? 'image/jpeg' : `image/${logoFormat}`;
  return `data:${mime};base64,${value}`;
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read logo file'));
    reader.onload = () =>
      typeof reader.result === 'string'
        ? resolve(reader.result)
        : reject(new Error('Failed to read logo file'));
    reader.readAsDataURL(file);
  });
}

export const BrandingSettingsCard: Component<BrandingSettingsCardProps> = (props) => {
  const [fileError, setFileError] = createSignal('');
  const preview = () => brandingLogoPreview(props.logoBase64(), props.logoFormat());

  const markChanged = () => props.setHasUnsavedChanges(true);

  const handleLogoFile = async (file: File | undefined) => {
    setFileError('');
    if (!file) return;

    const format = brandLogoFormatForFile(file);
    if (!format) {
      setFileError('Choose a PNG, JPEG, or GIF image.');
      return;
    }
    if (file.size > BRAND_LOGO_MAX_BYTES) {
      setFileError('Logo files must be 36 KB or smaller.');
      return;
    }

    try {
      props.setLogoBase64(await readFileAsDataURL(file));
      props.setLogoFormat(format);
      markChanged();
    } catch {
      setFileError('Pulse could not read that logo file.');
    }
  };

  const clearLogo = () => {
    props.setLogoBase64('');
    props.setLogoFormat('');
    setFileError('');
    markChanged();
  };

  return (
    <Show
      when={hasFeature('white_label')}
      fallback={
        <FeatureGateSection
          title="Application branding"
          body="Use a custom logo and name across the Pulse header and generated reports."
          upgradeDestination={getUpgradeActionDestination('white_label')}
          showUpgradePrompts={!presentationPolicyHidesUpgradePrompts()}
          icon={<ImageIcon class="h-5 w-5" />}
        />
      }
    >
      <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,0.8fr)]">
        <div class="space-y-4">
          <div>
            <label for="application-brand-name" class={formLabel}>
              Application name
            </label>
            <input
              id="application-brand-name"
              class={formControl}
              maxlength={120}
              value={props.displayName()}
              placeholder="Pulse"
              onInput={(event) => {
                props.setDisplayName(event.currentTarget.value);
                markChanged();
              }}
            />
            <p class={formHelpText}>
              Replaces “Pulse” in the page header and browser title. Leave blank to keep the default
              name, or to show an uploaded banner by itself.
            </p>
          </div>

          <div>
            <label for="application-brand-logo" class={formLabel}>
              Header logo
            </label>
            <input
              id="application-brand-logo"
              type="file"
              accept=".png,.jpg,.jpeg,.gif,image/png,image/jpeg,image/gif"
              class="block w-full text-sm text-muted file:mr-3 file:rounded-md file:border file:border-border file:bg-surface file:px-3 file:py-2 file:text-sm file:font-medium file:text-base-content hover:file:bg-surface-hover"
              onChange={(event) => void handleLogoFile(event.currentTarget.files?.[0])}
            />
            <p class={formHelpText}>
              PNG, JPEG, or GIF, up to 36 KB. Transparent or dark-background banner logos work best.
            </p>
            <Show when={fileError()}>
              <p class="mt-2 text-xs text-error" role="alert">
                {fileError()}
              </p>
            </Show>
          </div>
        </div>

        <div class="flex min-h-28 flex-col justify-between gap-4 rounded-md border border-border bg-base p-4">
          <div>
            <p class="text-xs font-medium uppercase tracking-wide text-muted">Header preview</p>
            <div class="mt-3 flex min-h-10 items-center justify-center gap-2 overflow-hidden rounded bg-surface px-3 py-2">
              <Show
                when={preview()}
                fallback={
                  <span class="flex h-5 w-5 items-center justify-center rounded-full bg-blue-600 text-[10px] text-white">
                    ●
                  </span>
                }
              >
                {(logo) => (
                  <img
                    src={logo()}
                    alt=""
                    class="max-h-8 max-w-[12rem] object-contain"
                    data-testid="branding-logo-preview"
                  />
                )}
              </Show>
              <Show when={props.displayName().trim() || !preview()}>
                <span class="truncate text-lg font-medium text-base-content">
                  {props.displayName().trim() || 'Pulse'}
                </span>
              </Show>
            </div>
          </div>

          <Show when={props.logoBase64()}>
            <div class="flex justify-end">
              <Button variant="secondary" size="sm" class="gap-2" onClick={clearLogo}>
                <Trash2 class="h-4 w-4" />
                Remove logo
              </Button>
            </div>
          </Show>
        </div>
      </div>
    </Show>
  );
};
