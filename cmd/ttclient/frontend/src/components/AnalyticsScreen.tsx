import { useTranslation } from "../i18n";
import { fmtBytes } from "../utils";
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";

export function AnalyticsScreen({ history, onCancel }: { history: { rx: number; tx: number }[]; onCancel: () => void }) {
  const { t } = useTranslation();

  const data = history.map((h, i) => ({
    time: i,
    download: h.rx,
    upload: h.tx,
  }));

  return (
    <div className="flex flex-col h-full bg-transparent text-fg-primary p-6 animate-fade-in">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-xs uppercase tracking-wide text-fg-faint font-medium">{t.analytics}</h2>
      </div>
      <p className="text-xs text-fg-muted mb-6">{t.analytics_hint}</p>

      <div className="flex-1 w-full min-h-[250px] bg-bg-subtle/50 backdrop-blur-md border border-border rounded-xl p-4">
        {data.length < 2 ? (
          <div className="h-full flex items-center justify-center text-fg-faint text-sm">
            No data yet. Connect to a profile first.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 10, right: 0, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="colorRx" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.8}/>
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0}/>
                </linearGradient>
                <linearGradient id="colorTx" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.8}/>
                  <stop offset="95%" stopColor="#f59e0b" stopOpacity={0}/>
                </linearGradient>
              </defs>
              <XAxis dataKey="time" hide />
              <YAxis hide domain={['auto', 'auto']} />
              <Tooltip
                contentStyle={{ backgroundColor: '#181818', border: '1px solid #27272a', borderRadius: '8px', fontSize: '12px' }}
                formatter={(val: any) => [`${fmtBytes(Number(val) || 0)}/s`, '']}
                labelFormatter={() => ''}
              />
              <Area type="monotone" dataKey="download" stroke="#10b981" fillOpacity={1} fill="url(#colorRx)" />
              <Area type="monotone" dataKey="upload" stroke="#f59e0b" fillOpacity={1} fill="url(#colorTx)" />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      <div className="flex justify-end gap-2 mt-6">
        <button onClick={onCancel} className="btn-primary px-6 py-2 rounded-full text-xs font-medium">Close</button>
      </div>
    </div>
  );
}
