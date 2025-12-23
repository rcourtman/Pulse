import { Component, createSignal, createEffect, For, Show, createMemo } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { apiFetch } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

interface Note {
    id: string;
    category: string;
    title: string;
    content: string;
    created_at: string;
    updated_at: string;
}

interface GuestKnowledge {
    guest_id: string;
    guest_name: string;
    guest_type: string;
    notes: Note[];
    updated_at: string;
}

interface GuestNotesProps {
    guestId: string;
    guestName?: string;
    guestType?: string;
    customUrl?: string;
    onCustomUrlUpdate?: (guestId: string, url: string) => void;
}

const CATEGORY_LABELS: Record<string, string> = {
    service: 'Service',
    path: 'Path',
    config: 'Config',
    credential: 'Credential',
    learning: 'Learning',
};

const CATEGORY_ICONS: Record<string, string> = {
    service: '⚙️',
    path: '📁',
    config: '📋',
    credential: '🔐',
    learning: '💡',
};

const CATEGORY_COLORS: Record<string, string> = {
    service: 'bg-blue-500/20 border-blue-500/30 text-blue-300',
    path: 'bg-amber-500/20 border-amber-500/30 text-amber-300',
    config: 'bg-purple-500/20 border-purple-500/30 text-purple-300',
    credential: 'bg-red-500/20 border-red-500/30 text-red-300',
    learning: 'bg-green-500/20 border-green-500/30 text-green-300',
};

const CATEGORY_OPTIONS = ['service', 'path', 'config', 'credential', 'learning'];

// Quick templates for common note types
const TEMPLATES = [
    { category: 'credential', title: 'Admin Password', placeholder: 'Enter admin password...' },
    { category: 'credential', title: 'SSH Key', placeholder: 'Paste SSH private key or fingerprint...' },
    { category: 'credential', title: 'API Key', placeholder: 'Enter API key...' },
    { category: 'path', title: 'Config Directory', placeholder: '/path/to/config' },
    { category: 'path', title: 'Data Directory', placeholder: '/path/to/data' },
    { category: 'path', title: 'Log Location', placeholder: '/var/log/service.log' },
    { category: 'service', title: 'Web Interface', placeholder: 'http://localhost:8080' },
    { category: 'service', title: 'Database', placeholder: 'PostgreSQL on port 5432' },
    { category: 'config', title: 'Port Number', placeholder: '8080' },
    { category: 'config', title: 'Environment', placeholder: 'production' },
];

// Format relative time
const formatRelativeTime = (dateStr: string): string => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMinutes = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMinutes < 1) return 'just now';
    if (diffMinutes < 60) return `${diffMinutes}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
};

export const GuestNotes: Component<GuestNotesProps> = (props) => {
    const [knowledge, setKnowledge] = createSignal<GuestKnowledge | null>(null);
    const [isLoading, setIsLoading] = createSignal(false);
    const [isExpanded, setIsExpanded] = createSignal(false);
    const [showAddForm, setShowAddForm] = createSignal(false);
    const [showTemplates, setShowTemplates] = createSignal(false);
    const [showActions, setShowActions] = createSignal(false);
    const [editingNote, setEditingNote] = createSignal<Note | null>(null);
    const [searchQuery, setSearchQuery] = createSignal('');
    const [filterCategory, setFilterCategory] = createSignal<string>('');
    const [showCredentials, setShowCredentials] = createSignal<Set<string>>(new Set());
    const [deleteConfirmId, setDeleteConfirmId] = createSignal<string | null>(null);
    const [clearConfirm, setClearConfirm] = createSignal(false);
    const [isImporting, setIsImporting] = createSignal(false);

    // Form state
    const [category, setCategory] = createSignal('learning');
    const [title, setTitle] = createSignal('');
    const [content, setContent] = createSignal('');

    // Guest URL state
    const [guestUrl, setGuestUrl] = createSignal(props.customUrl || '');
    const [isEditingUrl, setIsEditingUrl] = createSignal(false);
    const [isSavingUrl, setIsSavingUrl] = createSignal(false);

    // Sync URL from props
    createEffect(() => {
        setGuestUrl(props.customUrl || '');
    });

    // File input ref for import
    let fileInputRef: HTMLInputElement | undefined;

    // Fetch knowledge when guestId changes
    createEffect(() => {
        const guestId = props.guestId;
        if (guestId) {
            loadKnowledge(guestId);
        }
    });

    const loadKnowledge = async (guestId: string) => {
        setIsLoading(true);
        try {
            // Fetch knowledge and metadata in parallel
            const [knowledgeResponse, metadataResponse] = await Promise.all([
                apiFetch(`/api/ai/knowledge?guest_id=${encodeURIComponent(guestId)}`),
                apiFetch(`/api/guests/metadata/${encodeURIComponent(guestId)}`),
            ]);

            if (knowledgeResponse.ok) {
                const data = await knowledgeResponse.json();
                setKnowledge(data);
            }

            // Load customUrl from metadata if not provided via props
            if (metadataResponse.ok) {
                const metadata = await metadataResponse.json();
                if (metadata.customUrl && !props.customUrl) {
                    setGuestUrl(metadata.customUrl);
                }
            }
        } catch (error) {
            logger.error('Failed to load guest knowledge:', error);
        } finally {
            setIsLoading(false);
        }
    };

    const saveGuestUrl = async () => {
        const url = guestUrl().trim();
        setIsSavingUrl(true);
        try {
            const response = await apiFetch(`/api/guests/metadata/${encodeURIComponent(props.guestId)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ customUrl: url }),
            });
            if (response.ok) {
                notificationStore.success(url ? 'Guest URL saved' : 'Guest URL cleared');
                setIsEditingUrl(false);
                props.onCustomUrlUpdate?.(props.guestId, url);
            } else {
                notificationStore.error('Failed to save guest URL');
            }
        } catch (error) {
            logger.error('Failed to save guest URL:', error);
            notificationStore.error('Failed to save guest URL');
        } finally {
            setIsSavingUrl(false);
        }
    };

    const saveNote = async () => {
        if (!title().trim() || !content().trim()) return;

        try {
            const response = await apiFetch('/api/ai/knowledge/save', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    guest_id: props.guestId,
                    guest_name: props.guestName || props.guestId,
                    guest_type: props.guestType || 'unknown',
                    category: category(),
                    title: title().trim(),
                    content: content().trim(),
                }),
            });

            if (response.ok) {
                notificationStore.success('Note saved');
                // Reset form
                setTitle('');
                setContent('');
                setShowAddForm(false);
                setShowTemplates(false);
                setEditingNote(null);
                // Reload knowledge
                loadKnowledge(props.guestId);
            } else {
                notificationStore.error('Failed to save note');
            }
        } catch (error) {
            logger.error('Failed to save note:', error);
            notificationStore.error('Failed to save note');
        }
    };

    const deleteNote = async (noteId: string) => {
        try {
            const response = await apiFetch('/api/ai/knowledge/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    guest_id: props.guestId,
                    note_id: noteId,
                }),
            });

            if (response.ok) {
                notificationStore.success('Note deleted');
                setDeleteConfirmId(null);
                loadKnowledge(props.guestId);
            } else {
                notificationStore.error('Failed to delete note');
            }
        } catch (error) {
            logger.error('Failed to delete note:', error);
            notificationStore.error('Failed to delete note');
        }
    };

    const exportNotes = async () => {
        try {
            const response = await apiFetch(`/api/ai/knowledge/export?guest_id=${encodeURIComponent(props.guestId)}`);
            if (response.ok) {
                const data = await response.json();
                const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = `pulse-notes-${props.guestName || props.guestId}.json`;
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                URL.revokeObjectURL(url);
                notificationStore.success('Notes exported');
                setShowActions(false);
            } else {
                notificationStore.error('Failed to export notes');
            }
        } catch (error) {
            logger.error('Failed to export notes:', error);
            notificationStore.error('Failed to export notes');
        }
    };

    const handleImportFile = async (event: Event) => {
        const target = event.target as HTMLInputElement;
        const file = target.files?.[0];
        if (!file) return;

        setIsImporting(true);
        try {
            const text = await file.text();
            const data = JSON.parse(text);

            // Add merge flag and ensure guest_id matches current
            const importData = {
                ...data,
                guest_id: props.guestId,
                guest_name: props.guestName || data.guest_name,
                guest_type: props.guestType || data.guest_type,
                merge: true, // Merge with existing notes
            };

            const response = await apiFetch('/api/ai/knowledge/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(importData),
            });

            if (response.ok) {
                const result = await response.json();
                notificationStore.success(`Imported ${result.imported} of ${result.total} notes`);
                loadKnowledge(props.guestId);
                setShowActions(false);
            } else {
                const errorText = await response.text();
                notificationStore.error('Import failed: ' + errorText);
            }
        } catch (error) {
            logger.error('Failed to import notes:', error);
            notificationStore.error('Failed to parse import file');
        } finally {
            setIsImporting(false);
            // Reset file input
            if (fileInputRef) {
                fileInputRef.value = '';
            }
        }
    };

    const clearAllNotes = async () => {
        try {
            const response = await apiFetch('/api/ai/knowledge/clear', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    guest_id: props.guestId,
                    confirm: true,
                }),
            });

            if (response.ok) {
                const result = await response.json();
                notificationStore.success(`Cleared ${result.deleted} notes`);
                setClearConfirm(false);
                setShowActions(false);
                loadKnowledge(props.guestId);
            } else {
                notificationStore.error('Failed to clear notes');
            }
        } catch (error) {
            logger.error('Failed to clear notes:', error);
            notificationStore.error('Failed to clear notes');
        }
    };

    const startEdit = (note: Note) => {
        setEditingNote(note);
        setCategory(note.category);
        setTitle(note.title);
        setContent(note.content);
        setShowAddForm(true);
        setShowTemplates(false);
    };

    const useTemplate = (template: { category: string; title: string; placeholder: string }) => {
        setCategory(template.category);
        setTitle(template.title);
        setContent('');
        setShowTemplates(false);
        setShowAddForm(true);
    };

    const cancelEdit = () => {
        setEditingNote(null);
        setTitle('');
        setContent('');
        setShowAddForm(false);
        setShowTemplates(false);
    };

    const copyToClipboard = async (text: string, label: string) => {
        try {
            await navigator.clipboard.writeText(text);
            notificationStore.success(`${label} copied to clipboard`);
        } catch {
            notificationStore.error('Failed to copy to clipboard');
        }
    };

    const toggleCredentialVisibility = (noteId: string) => {
        const current = showCredentials();
        const updated = new Set(current);
        if (updated.has(noteId)) {
            updated.delete(noteId);
        } else {
            updated.add(noteId);
        }
        setShowCredentials(updated);
    };

    const maskCredential = (content: string): string => {
        // Mask most of the content, showing only first 2 and last 2 chars
        if (content.length <= 6) {
            return '••••••';
        }
        return content.slice(0, 2) + '•'.repeat(Math.min(content.length - 4, 12)) + content.slice(-2);
    };

    const notes = () => knowledge()?.notes || [];
    const hasNotes = () => notes().length > 0;

    // Filtered notes based on search and category
    const filteredNotes = createMemo(() => {
        let result = notes();

        // Filter by category
        const catFilter = filterCategory();
        if (catFilter) {
            result = result.filter(n => n.category === catFilter);
        }

        // Filter by search query
        const query = searchQuery().toLowerCase();
        if (query) {
            result = result.filter(n =>
                n.title.toLowerCase().includes(query) ||
                n.content.toLowerCase().includes(query) ||
                CATEGORY_LABELS[n.category]?.toLowerCase().includes(query)
            );
        }

        // Sort by updated_at descending
        return result.sort((a, b) =>
            new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
        );
    });

    // Group notes by category for summary
    const notesByCategory = createMemo(() => {
        const grouped: Record<string, number> = {};
        for (const note of notes()) {
            grouped[note.category] = (grouped[note.category] || 0) + 1;
        }
        return grouped;
    });

    return (
        <div class="border-t border-gray-700 pt-2 mt-2">
            {/* Hidden file input for import */}
            <input
                ref={fileInputRef}
                type="file"
                accept=".json"
                class="hidden"
                onChange={handleImportFile}
            />

            {/* Header with expand/collapse */}
            <button
                onClick={() => setIsExpanded(!isExpanded())}
                class="flex items-center justify-between w-full text-left px-2 py-1 text-sm hover:bg-gray-700/50 rounded transition-colors"
            >
                <span class="flex items-center gap-2">
                    <svg class={`w-3 h-3 transition-transform ${isExpanded() ? 'rotate-90' : ''}`} fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
                    </svg>
                    <span class="text-gray-300 font-medium">Saved Notes</span>
                    <Show when={hasNotes()}>
                        <span class="text-xs text-gray-500">({notes().length})</span>
                    </Show>
                </span>
                <Show when={isLoading()}>
                    <span class="text-xs text-gray-500">Loading...</span>
                </Show>
            </button>

            {/* Expandable content */}
            <Show when={isExpanded()}>
                <div class="mt-2 space-y-2 px-2">
                    {/* Guest URL field */}
                    <div class="bg-gray-800/50 rounded p-2 border border-gray-700">
                        <div class="flex items-center justify-between mb-1">
                            <span class="text-xs font-medium text-gray-400 flex items-center gap-1">
                                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                                </svg>
                                Guest URL
                            </span>
                            <Show when={guestUrl() && !isEditingUrl()}>
                                <a
                                    href={guestUrl()}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    class="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1"
                                >
                                    Open
                                    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                                    </svg>
                                </a>
                            </Show>
                        </div>
                        <Show when={isEditingUrl()} fallback={
                            <div class="flex items-center gap-2">
                                <Show when={guestUrl()} fallback={
                                    <span class="text-xs text-gray-500 italic">No URL set</span>
                                }>
                                    <span class="text-xs text-gray-300 break-all font-mono">{guestUrl()}</span>
                                </Show>
                                <button
                                    onClick={() => setIsEditingUrl(true)}
                                    class="text-xs text-blue-400 hover:text-blue-300 ml-auto"
                                >
                                    {guestUrl() ? 'Edit' : 'Add'}
                                </button>
                            </div>
                        }>
                            <div class="space-y-2">
                                <input
                                    type="text"
                                    value={guestUrl()}
                                    onInput={(e) => setGuestUrl(e.target.value)}
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter') {
                                            e.preventDefault();
                                            saveGuestUrl();
                                        } else if (e.key === 'Escape') {
                                            e.preventDefault();
                                            setGuestUrl(props.customUrl || '');
                                            setIsEditingUrl(false);
                                        }
                                    }}
                                    placeholder="https://192.168.1.100:8080"
                                    class="w-full bg-gray-700 text-xs text-gray-200 rounded px-2 py-1.5 border border-gray-600 placeholder-gray-500 font-mono"
                                    autofocus
                                />
                                <div class="flex gap-2 justify-end">
                                    <button
                                        onClick={() => {
                                            setGuestUrl(props.customUrl || '');
                                            setIsEditingUrl(false);
                                        }}
                                        class="text-xs px-2 py-1 text-gray-400 hover:text-gray-200"
                                    >
                                        Cancel
                                    </button>
                                    <button
                                        onClick={saveGuestUrl}
                                        disabled={isSavingUrl()}
                                        class="text-xs px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-500 disabled:opacity-50"
                                    >
                                        {isSavingUrl() ? 'Saving...' : 'Save'}
                                    </button>
                                </div>
                            </div>
                        </Show>
                    </div>

                    {/* Search and filter bar - only show if there are notes */}
                    <Show when={hasNotes()}>
                        <div class="flex gap-2 mb-2">
                            <input
                                type="text"
                                placeholder="Search notes..."
                                value={searchQuery()}
                                onInput={(e) => setSearchQuery(e.target.value)}
                                class="flex-1 bg-gray-700 text-xs text-gray-200 rounded px-2 py-1 border border-gray-600 placeholder-gray-500"
                            />
                            <select
                                value={filterCategory()}
                                onChange={(e) => setFilterCategory(e.target.value)}
                                class="bg-gray-700 text-xs text-gray-200 rounded px-2 py-1 border border-gray-600"
                            >
                                <option value="">All</option>
                                <For each={CATEGORY_OPTIONS}>
                                    {(cat) => (
                                        <Show when={notesByCategory()[cat]}>
                                            <option value={cat}>{CATEGORY_ICONS[cat]} {CATEGORY_LABELS[cat]} ({notesByCategory()[cat]})</option>
                                        </Show>
                                    )}
                                </For>
                            </select>
                        </div>
                    </Show>

                    {/* Notes list */}
                    <Show when={hasNotes()} fallback={
                        <p class="text-xs text-gray-500 italic">No saved notes yet. Add notes to remember passwords, paths, and configs.</p>
                    }>
                        <Show when={filteredNotes().length > 0} fallback={
                            <p class="text-xs text-gray-500 italic">No notes match your search.</p>
                        }>
                            <div class="space-y-1.5 max-h-48 overflow-y-auto">
                                <For each={filteredNotes()}>
                                    {(note) => (
                                        <div class={`group rounded border px-2 py-1.5 text-xs ${CATEGORY_COLORS[note.category] || 'bg-gray-800/50 border-gray-700'}`}>
                                            <div class="flex items-start justify-between gap-2">
                                                <div class="flex-1 min-w-0">
                                                    <div class="flex items-center gap-1.5">
                                                        <span class="text-sm" title={CATEGORY_LABELS[note.category]}>{CATEGORY_ICONS[note.category]}</span>
                                                        <span class="text-gray-200 font-medium">{note.title}</span>
                                                        <span class="text-gray-500 text-[10px]" title={new Date(note.updated_at).toLocaleString()}>
                                                            {formatRelativeTime(note.updated_at)}
                                                        </span>
                                                    </div>
                                                    <div class="flex items-center gap-1 mt-0.5">
                                                        {/* Content - mask if credential and not revealed */}
                                                        <Show when={note.category === 'credential' && !showCredentials().has(note.id)}
                                                            fallback={
                                                                <p class="text-gray-300 break-words font-mono text-[11px]">{note.content}</p>
                                                            }>
                                                            <p class="text-gray-400 break-words font-mono text-[11px]">{maskCredential(note.content)}</p>
                                                        </Show>
                                                    </div>
                                                </div>
                                                <div class="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
                                                    {/* Show/hide for credentials */}
                                                    <Show when={note.category === 'credential'}>
                                                        <button
                                                            onClick={() => toggleCredentialVisibility(note.id)}
                                                            class="text-gray-400 hover:text-yellow-400 p-0.5"
                                                            title={showCredentials().has(note.id) ? 'Hide' : 'Show'}
                                                        >
                                                            <Show when={showCredentials().has(note.id)} fallback={
                                                                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" /></svg>
                                                            }>
                                                                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" /></svg>
                                                            </Show>
                                                        </button>
                                                    </Show>
                                                    {/* Copy button */}
                                                    <button
                                                        onClick={() => copyToClipboard(note.content, note.title)}
                                                        class="text-gray-400 hover:text-green-400 p-0.5"
                                                        title="Copy content"
                                                    >
                                                        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" /></svg>
                                                    </button>
                                                    {/* Edit button */}
                                                    <button
                                                        onClick={() => startEdit(note)}
                                                        class="text-gray-400 hover:text-blue-400 p-0.5"
                                                        title="Edit"
                                                    >
                                                        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
                                                    </button>
                                                    {/* Delete button with confirmation */}
                                                    <Show when={deleteConfirmId() === note.id} fallback={
                                                        <button
                                                            onClick={() => setDeleteConfirmId(note.id)}
                                                            class="text-gray-400 hover:text-red-400 p-0.5"
                                                            title="Delete"
                                                        >
                                                            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                                                        </button>
                                                    }>
                                                        <div class="flex items-center gap-1 bg-red-900/50 rounded px-1">
                                                            <span class="text-red-300 text-[10px]">Delete?</span>
                                                            <button
                                                                onClick={() => deleteNote(note.id)}
                                                                class="text-red-400 hover:text-red-300 p-0.5 font-bold"
                                                                title="Confirm delete"
                                                            >
                                                                ✓
                                                            </button>
                                                            <button
                                                                onClick={() => setDeleteConfirmId(null)}
                                                                class="text-gray-400 hover:text-gray-300 p-0.5"
                                                                title="Cancel"
                                                            >
                                                                ✗
                                                            </button>
                                                        </div>
                                                    </Show>
                                                </div>
                                            </div>
                                        </div>
                                    )}
                                </For>
                            </div>
                        </Show>
                    </Show>

                    {/* Template picker */}
                    <Show when={showTemplates()}>
                        <div class="bg-gray-800/80 rounded p-2 border border-gray-700">
                            <div class="flex items-center justify-between mb-2">
                                <span class="text-xs font-medium text-gray-300">Quick Templates</span>
                                <button onClick={() => setShowTemplates(false)} class="text-gray-400 hover:text-gray-200 text-xs">✕</button>
                            </div>
                            <div class="grid grid-cols-2 gap-1">
                                <For each={TEMPLATES}>
                                    {(template) => (
                                        <button
                                            onClick={() => useTemplate(template)}
                                            class={`text-left px-2 py-1 rounded text-[10px] border ${CATEGORY_COLORS[template.category]} hover:opacity-80 transition-opacity`}
                                        >
                                            {CATEGORY_ICONS[template.category]} {template.title}
                                        </button>
                                    )}
                                </For>
                            </div>
                        </div>
                    </Show>

                    {/* Actions menu (Export/Import/Clear) */}
                    <Show when={showActions()}>
                        <div class="bg-gray-800/80 rounded p-2 border border-gray-700 space-y-1">
                            <div class="flex items-center justify-between mb-1">
                                <span class="text-xs font-medium text-gray-300">Actions</span>
                                <button onClick={() => { setShowActions(false); setClearConfirm(false); }} class="text-gray-400 hover:text-gray-200 text-xs">✕</button>
                            </div>
                            <button
                                onClick={exportNotes}
                                disabled={!hasNotes()}
                                class="w-full text-left px-2 py-1.5 rounded text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-300 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                            >
                                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                                Export Notes (JSON)
                            </button>
                            <button
                                onClick={() => fileInputRef?.click()}
                                disabled={isImporting()}
                                class="w-full text-left px-2 py-1.5 rounded text-xs bg-green-600/20 hover:bg-green-600/30 text-green-300 disabled:opacity-50 flex items-center gap-2"
                            >
                                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" /></svg>
                                {isImporting() ? 'Importing...' : 'Import Notes (Merge)'}
                            </button>
                            <Show when={clearConfirm()} fallback={
                                <button
                                    onClick={() => setClearConfirm(true)}
                                    disabled={!hasNotes()}
                                    class="w-full text-left px-2 py-1.5 rounded text-xs bg-red-600/20 hover:bg-red-600/30 text-red-300 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                                >
                                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                                    Clear All Notes
                                </button>
                            }>
                                <div class="bg-red-900/50 rounded p-2 flex items-center justify-between">
                                    <span class="text-red-300 text-[10px]">Delete all {notes().length} notes?</span>
                                    <div class="flex gap-1">
                                        <button onClick={clearAllNotes} class="px-2 py-0.5 bg-red-600 hover:bg-red-500 text-white rounded text-[10px]">Yes, clear all</button>
                                        <button onClick={() => setClearConfirm(false)} class="px-2 py-0.5 bg-gray-600 hover:bg-gray-500 text-white rounded text-[10px]">Cancel</button>
                                    </div>
                                </div>
                            </Show>
                        </div>
                    </Show>

                    {/* Add/Edit form */}
                    <Show when={showAddForm()}>
                        <div class="bg-gray-800/80 rounded p-2 space-y-2 border border-gray-700">
                            <div class="flex items-center justify-between mb-1">
                                <span class="text-xs font-medium text-gray-300">
                                    {editingNote() ? 'Edit Note' : 'Add Note'}
                                </span>
                                <Show when={editingNote()}>
                                    <span class="text-[10px] text-gray-500">ID: {editingNote()?.id}</span>
                                </Show>
                            </div>
                            <div class="flex gap-2">
                                <select
                                    value={category()}
                                    onChange={(e) => setCategory(e.target.value)}
                                    class="bg-gray-700 text-xs text-gray-200 rounded px-2 py-1.5 border border-gray-600"
                                >
                                    <For each={CATEGORY_OPTIONS}>
                                        {(cat) => <option value={cat}>{CATEGORY_ICONS[cat]} {CATEGORY_LABELS[cat]}</option>}
                                    </For>
                                </select>
                                <input
                                    type="text"
                                    placeholder="Title (e.g., 'Admin Password')"
                                    value={title()}
                                    onInput={(e) => setTitle(e.target.value)}
                                    class="flex-1 bg-gray-700 text-xs text-gray-200 rounded px-2 py-1.5 border border-gray-600 placeholder-gray-500"
                                />
                            </div>
                            <textarea
                                placeholder={category() === 'credential'
                                    ? "Enter credential value (stored encrypted)..."
                                    : "Content..."}
                                value={content()}
                                onInput={(e) => setContent(e.target.value)}
                                class="w-full bg-gray-700 text-xs text-gray-200 rounded px-2 py-1.5 border border-gray-600 resize-none placeholder-gray-500 font-mono"
                                rows={2}
                            />
                            <Show when={category() === 'credential'}>
                                <p class="text-[10px] text-amber-400 flex items-center gap-1">
                                    <span>🔐</span> Credentials are encrypted at rest and masked in the UI
                                </p>
                            </Show>
                            <div class="flex gap-2 justify-end">
                                <button
                                    onClick={cancelEdit}
                                    class="text-xs px-2 py-1 text-gray-400 hover:text-gray-200"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={saveNote}
                                    disabled={!title().trim() || !content().trim()}
                                    class="text-xs px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    {editingNote() ? 'Update' : 'Save'}
                                </button>
                            </div>
                        </div>
                    </Show>

                    {/* Action buttons row */}
                    <Show when={!showAddForm() && !showTemplates() && !showActions()}>
                        <div class="flex items-center gap-2 pt-1">
                            <button
                                onClick={() => setShowAddForm(true)}
                                class="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1"
                            >
                                <span>+</span> Add note
                            </button>
                            <button
                                onClick={() => setShowTemplates(true)}
                                class="text-xs text-purple-400 hover:text-purple-300 flex items-center gap-1"
                            >
                                <span>📝</span> Templates
                            </button>
                            <button
                                onClick={() => setShowActions(true)}
                                class="text-xs text-gray-400 hover:text-gray-300 flex items-center gap-1"
                            >
                                <span>⚙️</span> Actions
                            </button>
                        </div>
                    </Show>
                </div>
            </Show>
        </div>
    );
};
