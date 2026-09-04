import { useState } from "react";
import { useTranslation } from "../i18n";
import { ReadClipboard } from "../api";
import { ClipboardIcon } from "../icons";

export function PasteImportScreen({ onImport, onCancel }: { onImport: (content: string, name: string) => void; onCancel: () => void }) {
  const { t } = useTranslation();
  const [content, setContent] = useState("");
  const [name, setName] = useState("");

  async function handlePasteClipboard() {
    try {
      const text = await ReadClipboard();
      if (text) setContent(text);
    } catch {}
  }

  return (
    <div className="flex flex-col h-full bg-transparent text-fg-primary p-4 animate-fade-in">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs uppercase tracking-wide text-fg-faint font-medium">{t.paste_config}</h2>
      </div>
      <p className="text-xs text-fg-faint mb-3">{t.paste_hint}</p>
      <input
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="profile-name"
        className="w-full bg-bg-elevated border border-border rounded px-3 py-2 text-xs text-fg-secondary focus:outline-none focus:border-emerald-500 mb-2"
      />
      <div className="flex-1 flex flex-col min-h-0">
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder={'[endpoint]\naddress = "server:443"\nhostname = "example.com"\nusername = "user"\npassword = "pass"'}
          className="flex-1 w-full bg-bg-elevated border border-border rounded-lg p-3 text-xs font-mono text-fg-secondary resize-none focus:outline-none focus:border-emerald-500"
          spellCheck={false}
        />
      </div>
      <div className="flex justify-between gap-2 mt-3">
        <button onClick={handlePasteClipboard} className="btn-ghost px-3 py-2 rounded-md text-xs font-medium flex items-center gap-1.5">
          <ClipboardIcon className="w-3.5 h-3.5" /> {t.paste_clipboard}
        </button>
        <div className="flex gap-2">
          <button onClick={onCancel} className="btn-ghost px-4 py-2 rounded-md text-xs font-medium">{t.cancel}</button>
          <button onClick={() => onImport(content, name)} disabled={!content.trim()} className="btn-primary px-4 py-2 rounded-md text-xs font-medium bg-emerald-500 text-zinc-900 hover:bg-emerald-400 disabled:opacity-50">
            {t.import_text}
          </button>
        </div>
      </div>
    </div>
  );
}
