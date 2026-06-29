import { useEffect, useState } from "react";
import { api, Rule } from "../api";
import { Empty, Field, Icon, Modal, useConfirm, useToast } from "../ui";

export function Rules() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [path, setPath] = useState("");
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<{ rule: Rule; index: number } | null>(null);
  const toast = useToast();
  const { confirm, dialog } = useConfirm();

  async function load() {
    try {
      const r = await api.rules();
      setRules(r.rules ?? []);
      setPath(r.path);
    } catch (e) {
      toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function save(next: Rule[]) {
    try {
      await api.saveRules(next);
      toast({ type: "ok", text: "Rules saved" });
      setRules(next);
    } catch (e) {
      toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    }
  }

  async function del(i: number) {
    const ok = await confirm("Delete rule", `Rule #${i + 1} will be removed.`, true);
    if (!ok) return;
    save(rules.filter((_, idx) => idx !== i));
  }

  function move(i: number, dir: -1 | 1) {
    const next = [...rules];
    const j = i + dir;
    if (j < 0 || j >= next.length) return;
    [next[i], next[j]] = [next[j], next[i]];
    save(next);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-zinc-100">Routing rules</h1>
          <p className="mt-1 text-sm text-zinc-500">
            Filter connections by client IP (CIDR) and TLS ClientHello random before authentication.
            <span className="hidden sm:inline"> Evaluated top-to-bottom; first match wins. Default policy is allow.</span>
          </p>
        </div>
        <button onClick={() => setAdding(true)} className="btn-primary shrink-0">
          <Icon.Plus className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">Add rule</span>
          <span className="sm:hidden">Add</span>
        </button>
      </div>

      {path && (
        <div className="text-xs text-zinc-500">
          File: <code className="bg-bg-elevated px-1.5 py-0.5 rounded text-zinc-400">{path}</code>
        </div>
      )}

      {rules.length === 0 && !loading && (
        <Empty
          title="No rules configured"
          hint="All connections are allowed by default. Add rules to deny or whitelist specific clients."
          icon={<Icon.Shield className="w-8 h-8" />}
        />
      )}

      {rules.length > 0 && (
        <div className="card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-[11px] uppercase tracking-wide text-zinc-500">
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5 w-10">#</th>
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5">CIDR</th>
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5 hidden md:table-cell">Client random prefix</th>
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5">Action</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {rules.map((r, i) => (
                  <tr key={i} className="hover:bg-bg-elevated/40 transition-colors">
                    <td className="px-3 sm:px-4 py-3 text-zinc-500 tabular-nums">{i + 1}</td>
                    <td className="px-3 sm:px-4 py-3 font-mono text-xs text-zinc-200">
                      {r.cidr || <span className="text-zinc-600">any</span>}
                      <div className="md:hidden mt-1 text-[11px] text-zinc-500 truncate max-w-[180px]">
                        {r.client_random_prefix || "any random"}
                      </div>
                    </td>
                    <td className="px-3 sm:px-4 py-3 font-mono text-xs text-zinc-400 hidden md:table-cell">
                      {r.client_random_prefix || <span className="text-zinc-600">any</span>}
                    </td>
                    <td className="px-3 sm:px-4 py-3">
                      <ActionPill action={r.action} />
                    </td>
                    <td className="px-3 sm:px-4 py-3 text-right">
                      <div className="flex justify-end gap-1">
                        <button
                          onClick={() => move(i, -1)}
                          disabled={i === 0}
                          className="text-zinc-400 hover:text-zinc-100 p-1.5 rounded hover:bg-bg-elevated transition-colors disabled:opacity-30 disabled:hover:bg-transparent"
                          title="Move up"
                        >
                          <Icon.ArrowUp className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => move(i, 1)}
                          disabled={i === rules.length - 1}
                          className="text-zinc-400 hover:text-zinc-100 p-1.5 rounded hover:bg-bg-elevated transition-colors disabled:opacity-30 disabled:hover:bg-transparent"
                          title="Move down"
                        >
                          <Icon.ArrowDown className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => setEditing({ rule: r, index: i })}
                          className="text-zinc-400 hover:text-zinc-100 p-1.5 rounded hover:bg-bg-elevated transition-colors"
                          title="Edit"
                        >
                          <Icon.Edit className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => del(i)}
                          className="text-zinc-400 hover:text-red-400 p-1.5 rounded hover:bg-red-500/10 transition-colors"
                          title="Delete"
                        >
                          <Icon.Trash className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {adding && (
        <RuleModal
          onClose={() => setAdding(false)}
          onSave={(rule) => save([...rules, rule])}
          title="Add rule"
        />
      )}
      {editing && (
        <RuleModal
          initial={editing.rule}
          onClose={() => setEditing(null)}
          onSave={(rule) => {
            const next = [...rules];
            next[editing.index] = rule;
            save(next);
          }}
          title={`Edit rule #${editing.index + 1}`}
        />
      )}
      {dialog}
    </div>
  );
}

function ActionPill({ action }: { action: string }) {
  if (action === "allow") {
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
        <Icon.Check className="w-3 h-3" /> allow
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-red-500/10 text-red-400 border border-red-500/30">
      <Icon.X className="w-3 h-3" /> deny
    </span>
  );
}

function RuleModal(props: { title: string; initial?: Rule; onClose: () => void; onSave: (r: Rule) => void }) {
  const [cidr, setCidr] = useState(props.initial?.cidr ?? "");
  const [prefix, setPrefix] = useState(props.initial?.client_random_prefix ?? "");
  const [action, setAction] = useState<"allow" | "deny">(props.initial?.action ?? "deny");
  const [error, setError] = useState<string | null>(null);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!cidr && !prefix) {
      setError("Specify CIDR or client random prefix (or both)");
      return;
    }
    props.onSave({
      cidr: cidr.trim(),
      client_random_prefix: prefix.trim(),
      action,
    });
    props.onClose();
  }

  return (
    <Modal title={props.title} onClose={props.onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="CIDR" hint="IPv4 or IPv6 network. Leave empty to match any IP.">
          <input
            value={cidr}
            onChange={(e) => setCidr(e.target.value)}
            placeholder="192.168.0.0/16"
            className="input font-mono text-xs"
            autoFocus
          />
        </Field>
        <Field
          label="Client random prefix"
          hint='Hex. Use "ab12/ffff" for prefix+mask. Leave empty to match any ClientHello.'
        >
          <input
            value={prefix}
            onChange={(e) => setPrefix(e.target.value)}
            placeholder="ab12 or ab12/ff00"
            className="input font-mono text-xs"
          />
        </Field>
        <Field label="Action">
          <div className="grid grid-cols-2 gap-2">
            {(["allow", "deny"] as const).map((a) => (
              <button
                key={a}
                type="button"
                onClick={() => setAction(a)}
                className={`px-3 py-2 rounded-md text-sm border transition-colors ${
                  action === a
                    ? a === "allow"
                      ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/40"
                      : "bg-red-500/10 text-red-400 border-red-500/40"
                    : "bg-bg-elevated text-zinc-400 border-border hover:text-zinc-200"
                }`}
              >
                {a}
              </button>
            ))}
          </div>
        </Field>
        {error && (
          <div className="flex items-start gap-2 bg-red-500/10 border border-red-500/30 text-red-300 px-3 py-2 rounded-md text-xs">
            <Icon.Alert className="w-4 h-4 mt-0.5 shrink-0" />
            <span>{error}</span>
          </div>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={props.onClose} className="btn-ghost">
            Cancel
          </button>
          <button type="submit" className="btn-primary">
            Save
          </button>
        </div>
      </form>
    </Modal>
  );
}
