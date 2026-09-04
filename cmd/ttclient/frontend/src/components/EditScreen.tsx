import { useState, useEffect } from "react";
import { ReadProfileContent } from "../api";
import { Profile } from "../types";

export function EditScreen(props: { profile: Profile; onSave: (content: string) => Promise<void>; onCancel: () => void }) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    ReadProfileContent(props.profile.id)
      .then(setContent)
      .catch((e: any) => setError(String(e?.message ?? e)))
      .finally(() => setLoading(false));
  }, [props.profile.id]);

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      await props.onSave(content);
    } catch (e: any) {
      setError(String(e?.message ?? e));
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col h-full bg-transparent text-fg-primary p-4 animate-fade-in">
      {loading ? (
        <div className="flex-1 flex items-center justify-center text-fg-faint text-sm">Loading...</div>
      ) : (
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          className="flex-1 w-full bg-bg-elevated border border-border rounded-lg p-3 text-xs font-mono text-fg-secondary resize-none focus:outline-none focus:border-zinc-500 mb-4"
          spellCheck={false}
        />
      )}
      {error && <div className="text-red-400 text-xs mb-3 font-medium">{error}</div>}
      <div className="flex justify-end gap-2">
        <button onClick={props.onCancel} disabled={saving} className="btn-ghost px-4 py-2 rounded-md text-xs font-medium">Cancel</button>
        <button onClick={handleSave} disabled={saving || loading} className="btn-primary px-4 py-2 rounded-md text-xs font-medium bg-emerald-500 text-zinc-900 hover:bg-emerald-400 disabled:opacity-50">
          {saving ? "Saving..." : "Save"}
        </button>
      </div>
    </div>
  );
}
