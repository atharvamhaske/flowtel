import { FooterShader } from "./footer-shader";

const repo = "https://github.com/atharvamhaske/flowtel";

const btn =
  "group inline-flex items-center justify-center gap-2 border font-medium tracking-[-0.15px] transition-[transform,background-color,border-color] duration-150 ease-out active:scale-[0.97] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent motion-reduce:transition-none motion-reduce:active:scale-100 shadow-[inset_0_2px_1px_#ffffff40,inset_0_-3px_1px_#00000018,0_0_0_4px_#23232308,0_2px_3px_#17213a20,0_5px_10px_#17213a12] active:shadow-[inset_0_1px_3px_#00000024,inset_0_-1px_0_#ffffff20,0_0_0_4px_#23232308,0_1px_2px_#17213a12]";
const lg = "min-h-12 rounded-xl px-[18px] py-3 text-[14px]";
const primary = `${btn} border-accent-dark bg-accent text-paper hover:bg-[#d9520b]`;
const secondary = `${btn} border-[#c7c7c7] bg-[#f3f3f3] text-[#484848] hover:border-[#bdbdbd] hover:bg-[#eaeaea]`;

const kicker = "mb-[19px] text-[10px] uppercase leading-normal tracking-[0.08em] text-[#646464]";
const h2 = "mb-5 text-[27px] font-medium leading-[1.2] tracking-[-1.3px] sm:text-[34px]";
const body = "leading-[1.8] text-muted";
const section = "scroll-mt-5 border-t border-line px-5 py-10 sm:px-11 sm:py-[60px]";

const features = [
  {
    tag: "01",
    title: "Harness-layer traces",
    copy: "One trace per session, parented exactly the way the harness ran it: session, llm, tool, permission.",
  },
  {
    tag: "02",
    title: "Permission spans",
    copy: "When a harness asks to run something, the allow or deny decision, and who made it, lands in the trace.",
  },
  {
    tag: "03",
    title: "Plain OTLP out",
    copy: "OpenInference and OTel GenAI attributes on the same spans. No SDK, no proprietary fields, no raw payloads by default.",
  },
];

const compareRows = [
  { cap: "Layer it lives at", cells: ["Harness runtime", "App / SDK", "App / SDK", "App / SDK", "App / SDK"] },
  { cap: "Tool calls + permission decisions", cells: ["yes", "-", "-", "-", "-"] },
  { cap: "Vendor-neutral OTLP, no SDK", cells: ["yes", "-", "-", "-", "-"] },
  { cap: "Raw payloads off by default", cells: ["yes", "-", "-", "-", "-"] },
  { cap: "Evals, scoring, judges", cells: ["no, by design", "yes", "yes", "yes", "yes"] },
  { cap: "Works as a flowtel backend", cells: ["-", "yes", "yes", "yes", "yes"] },
];
const compareCols = ["Flowtel", "Braintrust", "Laminar", "Phoenix", "LangSmith"];

const footerCols = [
  {
    title: "Project",
    links: [
      { label: "GitHub", href: repo },
      { label: "SPEC.md", href: `${repo}/blob/main/SPEC.md` },
      { label: "Releases", href: `${repo}/releases` },
    ],
  },
  {
    title: "Standards",
    links: [
      { label: "OpenTelemetry", href: "https://opentelemetry.io" },
      { label: "OpenInference", href: "https://github.com/Arize-ai/openinference" },
      { label: "GenAI semconv", href: "https://opentelemetry.io/docs/specs/semconv/gen-ai/" },
    ],
  },
];

function Arrow() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 16 16"
      fill="none"
      aria-hidden
      className="transition-transform duration-150 ease-out group-hover:translate-x-[3px] motion-reduce:transition-none"
    >
      <path d="M3 8h10M9 4l4 4-4 4" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function Brand() {
  return (
    <a href="#" className="flex items-center gap-1.5 text-[#111]">
      <img src="/flowtel-mark.png" alt="" className="h-6 w-auto" />
      <span className="font-display text-[21px] tracking-[-0.8px] sm:text-[23px]">flowtel</span>
    </a>
  );
}

const link = "transition-colors duration-150 hover:text-accent hover:underline hover:underline-offset-4";

const widths = { 800: "max-w-[800px]", 900: "max-w-[900px]" };

export function Landing({ width }: { width: keyof typeof widths }) {
  return (
    <main className="min-h-dvh px-3 text-[16px] leading-[1.65] sm:px-8">
      <div className={`mx-auto ${widths[width]} border-x border-line bg-paper`}>
        <header className="flex h-14 items-center justify-between gap-5 border-b border-[#ededed] px-5 sm:px-11">
          <Brand />
          <nav className="flex items-center gap-3 text-[13px] tracking-[-0.2px] sm:gap-6">
            <a className={`${link} hidden sm:block`} href="#features">features</a>
            <a className={`${link} hidden sm:block`} href="#run">run</a>
            <a className={`${link} hidden sm:block`} href="#compare">compare</a>
            <a className={`${primary} min-h-9 rounded-lg px-3 py-1.5 text-[13px]`} href={repo}>
              GitHub <Arrow />
            </a>
          </nav>
        </header>

        {/* Hero */}
        <section className="px-5 pb-10 pt-12 text-center sm:px-11 sm:pb-16 sm:pt-[76px]">
          <p className={`${kicker} mb-1 animate-rise motion-reduce:animate-none`}>agent session to OTLP</p>
          <h1 className="relative isolate mx-auto mb-4 max-w-[14ch] animate-rise font-display text-[clamp(36px,9vw,58px)] leading-[1.1] tracking-[-2px] [animation-delay:60ms] motion-reduce:animate-none">
            make agent work{" "}
            <span className="relative whitespace-nowrap">
              <span
                aria-hidden
                className="absolute inset-x-0 bottom-[0.04em] top-[0.3em] -z-10 animate-highlight bg-accent-soft motion-reduce:animate-none"
              />
              legible.
            </span>
          </h1>
          <p className="mx-auto mb-7 max-w-[52ch] animate-rise text-[15px] leading-[1.8] text-muted [animation-delay:120ms] [text-wrap:pretty] motion-reduce:animate-none sm:text-[17px]">
            <strong className="font-medium text-ink">OpenTelemetry</strong> for what happens after the model answers. Flowtel turns{" "}
            <strong className="font-medium text-ink">coding-harness sessions</strong> into{" "}
            <strong className="font-medium text-ink">vendor-neutral OpenTelemetry</strong>: turns, model calls, tool calls and permission
            decisions.
          </p>
          <div className="mx-auto grid max-w-[440px] animate-rise gap-2.5 sm:flex sm:max-w-none sm:flex-wrap sm:justify-center [animation-delay:180ms] motion-reduce:animate-none">
            <a className={`${primary} ${lg}`} href={repo}>
              Star on GitHub <Arrow />
            </a>
            <a className={`${secondary} min-h-12 rounded-xl px-[18px] py-3 font-mono text-[13px]`} href="#run">
              $ flowctl pi run
            </a>
          </div>
          <div className="mt-6 flex animate-rise flex-wrap justify-center gap-x-5 gap-y-2 text-[12px] text-[#767676] [animation-delay:240ms] motion-reduce:animate-none">
            <span>no SDK dependency</span>
            <span>no proprietary attributes</span>
            <span>no raw payloads by default</span>
          </div>
          <div className="mx-auto mt-16 max-w-[60ch] animate-rise [animation-delay:300ms] motion-reduce:animate-none">
            <p className={`${kicker} mb-2`}>How it works</p>
            <p className="mb-2 text-[19px] font-medium tracking-[-0.4px] text-ink">From harness session to your backend.</p>
            <p className={`${body} text-[14px]`}>
              Pi sessions arrive as JSONL or live events, become one event model, render as OTel spans and metrics, and leave through
              a single OTLP pipeline to a collector that fans out to your trace, log and metric backends.
            </p>
          </div>
          <figure className="mt-6 animate-rise overflow-hidden rounded-xl border border-[#ededed] bg-paper p-2 [animation-delay:360ms] motion-reduce:animate-none sm:p-4">
            <img src="/flowtel.svg" alt="Flowtel architecture: harness sessions exported as OTLP traces, metrics and logs" width={2000} height={1137} className="h-auto w-full" />
          </figure>
        </section>

        {/* Features */}
        <section id="features" className={section}>
          <div className="max-w-[600px]">
            <p className={kicker}>Signals</p>
            <h2 className={h2}>Everything the harness did, as first-class telemetry.</h2>
            <p className={`${body} max-w-[500px]`}>
              Flowtel wraps the agent runtime instead of the model API, so it sees the repository work a model trace never shows.
            </p>
          </div>
          <div className="mt-8 grid gap-6 sm:mt-[42px] sm:grid-cols-3 sm:gap-[30px]">
            {features.map((f) => (
              <article key={f.title} className="border-t border-line pt-6">
                <span className="font-mono text-[11px] text-accent">{f.tag}</span>
                <h3 className="mb-2.5 mt-3 text-[17px] font-semibold tracking-[-0.3px]">{f.title}</h3>
                <p className={`${body} text-[14px]`}>{f.copy}</p>
              </article>
            ))}
          </div>
        </section>

        {/* Run */}
        <section id="run" className={`${section} grid items-center gap-6 bg-[#fcfcfc] sm:grid-cols-[1fr_1.15fr] sm:gap-11`}>
          <div>
            <p className={kicker}>Your harness. Your collector.</p>
            <h2 className={h2}>Pi today. Small adapters, easy to add.</h2>
            <p className={`${body} mb-5 text-[14px]`}>
              Point flowtel at any OTLP endpoint. Claude Code, opencode and omp adapters are planned.
            </p>
            <a className="inline-flex items-center gap-1.5 text-[13px] font-medium text-accent hover:underline hover:underline-offset-4" href={`${repo}/blob/main/ONBOARDING.md`}>
              Read the onboarding guide <Arrow />
            </a>
          </div>
          <div className="min-w-0 overflow-hidden rounded-lg border border-[#dedede] bg-paper">
            <div className="border-b border-line px-4 py-3 text-[12px] text-faint">terminal</div>
            <pre className="overflow-x-auto px-5 py-6 font-mono text-[12px] leading-[2.1] text-muted">
              <code>
                <span className="text-accent">FLOWTEL_OTLP_ENDPOINT</span>=http://collector:4318{"\n"}
                <span className="text-accent">FLOWTEL_ATTRIBUTE_PROFILE</span>=both{"\n"}
                <span className="text-accent">FLOWTEL_TAGS</span>=team=platform,env=ci{"\n"}
                <span className="text-ink">$ flowctl pi run</span>
              </code>
            </pre>
          </div>
        </section>

        {/* Compare */}
        <section id="compare" className={section}>
          <div className="max-w-[600px]">
            <p className={kicker}>Compare</p>
            <h2 className={h2}>Not a platform. It feeds them.</h2>
            <p className={`${body} max-w-[500px]`}>
              Eval platforms live at the app layer. Flowtel lives at the harness layer and exports plain OTLP, so every product below
              is a destination, not a competitor.
            </p>
          </div>
          <div className="mt-8 overflow-x-auto rounded-lg border border-line">
            <table className="w-full min-w-[620px] border-collapse text-left text-[13px]">
              <thead>
                <tr className="border-b border-line bg-[#fcfcfc]">
                  <th className="px-4 py-3 text-[12px] font-normal text-faint">Capability</th>
                  {compareCols.map((col, i) => (
                    <th key={col} className={`px-4 py-3 font-medium ${i === 0 ? "text-accent" : "text-ink"}`}>
                      {col}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {compareRows.map((row) => (
                  <tr key={row.cap} className="border-b border-line last:border-0">
                    <td className="px-4 py-3 text-muted">{row.cap}</td>
                    {row.cells.map((cell, i) => (
                      <td
                        key={i}
                        className={`px-4 py-3 font-mono text-[12px] ${cell === "yes" && i === 0 ? "text-accent" : cell === "-" ? "text-[#c7c7c7]" : "text-ink"}`}
                      >
                        {cell}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* Footer */}
        <footer className="relative isolate overflow-hidden border-t border-line px-5 pb-5 pt-8 sm:px-11 sm:pb-7 sm:pt-12">
          <FooterShader className="pointer-events-none absolute inset-x-0 bottom-0 -z-10 h-[75%] w-full [mask-image:linear-gradient(to_top,black_55%,transparent)]" />
          <div className="flex flex-col justify-between gap-6 sm:flex-row sm:gap-10">
            <div>
              <p className="mb-3 text-[22px] font-medium leading-[1.3] tracking-[-0.6px] sm:text-[24px]">
                Boring, portable, easy to trust.
              </p>
              <p className="text-[14px] leading-[1.6] text-muted">Vendor-neutral telemetry for coding-agent harnesses.</p>
            </div>
            <nav className="flex gap-16">
              {footerCols.map((col) => (
                <div key={col.title} className="flex flex-col items-start">
                  <span className="mb-2 text-[12px] text-faint">{col.title}</span>
                  {col.links.map((l) => (
                    <a key={l.label} className={`${link} inline-flex min-h-9 items-center text-[13px]`} href={l.href}>
                      {l.label}
                    </a>
                  ))}
                </div>
              ))}
            </nav>
          </div>
          <div className="select-none whitespace-nowrap pb-7 pt-16 text-center font-display text-[clamp(56px,17vw,136px)] leading-[1.15] tracking-[-0.055em] sm:pb-9 sm:pt-24">
            flowtel<span className="text-accent">.</span>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-ink/15 pt-6 text-[12px] text-ink">
            <span>MIT licensed, built in the open</span>
            <a className={link} href="https://x.com/atharvaxdevs">x / @atharvaxdevs</a>
          </div>
        </footer>
      </div>
    </main>
  );
}
