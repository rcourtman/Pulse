import { Component, createSignal } from 'solid-js';
import { showSuccess } from '@/utils/toast';
import { SettingsAPI } from '@/api/settings';
import type { WizardState } from '../SetupWizard';

interface FeaturesStepProps {
    state: WizardState;
    updateState: (updates: Partial<WizardState>) => void;
    onNext: () => void;
    onBack: () => void;
}

export const FeaturesStep: Component<FeaturesStepProps> = (props) => {
    const [aiEnabled, setAiEnabled] = createSignal(false);
    const [autoUpdates, setAutoUpdates] = createSignal(true);
    const [isSaving, setIsSaving] = createSignal(false);

    const features = [
        {
            id: 'ai',
            name: 'Pulse Assistant',
            icon: '🤖',
            desc: 'Guided troubleshooting with Patrol automation and auto-fix capabilities',
            enabled: aiEnabled,
            setEnabled: setAiEnabled,
            badge: 'New in 5.0',
        },
        {
            id: 'updates',
            name: 'Automatic Updates',
            icon: '🔄',
            desc: 'Keep Pulse up-to-date automatically',
            enabled: autoUpdates,
            setEnabled: setAutoUpdates,
            badge: null,
        },
    ];

    const handleContinue = async () => {
        setIsSaving(true);

        try {
            // Only save auto-update setting through SystemConfig
            // AI settings are configured separately via Settings → AI
            await SettingsAPI.updateSystemSettings({
                autoUpdateEnabled: autoUpdates(),
            });

            props.updateState({
                aiEnabled: aiEnabled(),
                autoUpdatesEnabled: autoUpdates(),
            });

            showSuccess('Preferences saved!');
            props.onNext();
        } catch (_error) {
            // Continue anyway - settings can be changed later
            props.onNext();
        } finally {
            setIsSaving(false);
        }
    };

    return (
        <div class="bg-white/10 backdrop-blur-xl rounded-2xl border border-white/20 overflow-hidden">
            <div class="p-6 border-b border-white/10">
                <h2 class="text-2xl font-bold text-white">Enable Features</h2>
                <p class="text-white/70 mt-1">Customize your Pulse experience</p>
            </div>

            <div class="p-6 space-y-4">
                {features.map((feature) => (
                    <button
                        onClick={() => feature.setEnabled(!feature.enabled())}
                        class={`w-full p-4 rounded-xl border transition-all text-left flex items-start gap-4 ${feature.enabled()
                            ? 'bg-blue-500/20 border-blue-400/40'
                            : 'bg-white/5 border-white/10 hover:bg-white/10'
                            }`}
                    >
                        <div class="text-3xl">{feature.icon}</div>
                        <div class="flex-1">
                            <div class="flex items-center gap-2">
                                <span class="text-white font-medium">{feature.name}</span>
                                {feature.badge && (
                                    <span class="px-2 py-0.5 bg-green-500/30 text-green-300 text-xs rounded-full">
                                        {feature.badge}
                                    </span>
                                )}
                            </div>
                            <p class="text-white/60 text-sm mt-1">{feature.desc}</p>
                        </div>
                        <div class={`w-12 h-7 rounded-full transition-all flex items-center px-1 ${feature.enabled() ? 'bg-blue-500' : 'bg-white/20'
                            }`}>
                            <div class={`w-5 h-5 rounded-full bg-white transition-transform ${feature.enabled() ? 'translate-x-5' : ''
                                }`} />
                        </div>
                    </button>
                ))}

                {/* Assistant info box */}
                <div class="bg-purple-500/10 border border-purple-400/20 rounded-xl p-4">
                    <div class="flex items-start gap-3">
                        <div class="text-2xl">✨</div>
                        <div>
                            <p class="text-white font-medium">Pulse Assistant & Patrol Features</p>
                            <p class="text-white/60 text-sm mt-1">
                                • Chat assistant for infrastructure questions<br />
                                • Patrol mode for proactive monitoring<br />
                                • Auto-fix for common issues<br />
                                • Predictive failure detection
                            </p>
                            <p class="text-white/40 text-xs mt-2">
                                Requires API key configuration in Settings → AI after setup
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Actions */}
            <div class="p-6 bg-black/20 flex gap-3">
                <button
                    onClick={props.onBack}
                    class="px-6 py-3 bg-white/10 hover:bg-white/20 text-white rounded-xl"
                >
                    ← Back
                </button>
                <button
                    onClick={handleContinue}
                    disabled={isSaving()}
                    class="flex-1 py-3 px-6 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white font-medium rounded-xl transition-all"
                >
                    {isSaving() ? 'Saving...' : 'Continue →'}
                </button>
            </div>
        </div>
    );
};
