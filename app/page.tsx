const mark = "font-semibold text-ink";

export default function Home() {
  return (
    <main className="grain min-h-dvh overflow-x-hidden overflow-y-auto bg-white px-4 py-5 text-ink sm:px-7 sm:py-7 md:px-10 md:py-8">
      <div className="mx-auto flex min-h-[calc(100dvh-40px)] w-full max-w-[720px] flex-col sm:min-h-[calc(100dvh-56px)] md:min-h-[calc(100dvh-64px)]">
        <header className="mb-8 flex items-start justify-between gap-4 sm:mb-10 sm:gap-8 md:mb-12">
          <img src="/flowtelwhite.png" alt="Flowtel" className="h-auto w-[116px] object-contain sm:w-[148px] md:w-[164px]" />
          <nav className="pt-1 text-right text-[10px] leading-4 text-black/60 sm:pt-2 sm:text-xs sm:leading-5 md:text-sm md:leading-6"><a className="transition-colors hover:text-orange" href="https://github.com/atharvamhaske/flowtel">github</a><span className="px-1.5 sm:px-2">·</span><a className="transition-colors hover:text-orange" href="https://x.com/atharvaxdevs">x / @atharvaxdevs</a></nav>
        </header>
        <section className="flex flex-1 flex-col items-center justify-start py-3 text-center sm:py-5 md:py-7">
          <h1 className="max-w-[660px] font-sans text-[clamp(2.75rem,8vw,5.5rem)] font-semibold leading-[.92] tracking-[-.065em] sm:text-[clamp(3.5rem,7vw,5.5rem)]">make agent work <span className="text-orange">legible.</span></h1>
          <p className="mt-6 max-w-[430px] text-[14px] leading-5 text-black/60 sm:mt-8 sm:text-[16px] sm:leading-6 md:mt-9 md:text-[18px] md:leading-7">Open telemetry for the part of AI work that happens after the model answers.</p>
          <div className="mt-7 max-w-[680px] space-y-0 text-left sm:mt-9 md:mt-10"><p className="text-justify text-[13px] leading-5 text-black/75 sm:text-[14px] sm:leading-6 md:text-[15px] md:leading-6">I built Flowtel because a model trace is only the middle of the story. It can tell you a request reached <span className={mark}>OpenAI, Anthropic, or vLLM</span>. It cannot tell you what the coding agent was doing around it, which repository it touched, which shell or file tool it called, or whether someone let it continue.</p><p className="text-justify text-[13px] leading-5 text-black/75 sm:text-[14px] sm:leading-6 md:text-[15px] md:leading-6">Flowtel sits at the <span className={mark}>harness layer</span>: Pi first, then Claude Code, OpenCode, OMP, and other agents as small adapters that are easy to test and replace. It turns sessions, turns, branches, model activity, tool calls, and <span className="font-semibold text-orange">permission decisions</span> into one vendor-neutral OpenTelemetry shape, with <span className={mark}>OpenInference and OTel GenAI</span> semantics on the same spans and bounded audit logs beside them.</p><p className="text-justify text-[13px] leading-5 text-black/75 sm:text-[14px] sm:leading-6 md:text-[15px] md:leading-6">Send those traces to Phoenix, Braintrust, or Laminar and the logs to Greptime or Parseable without rebuilding your instrumentation for every backend. This is for the team that needs a receipt when an agent changes code, and for the developer who wants to understand a messy session without sending every prompt and tool payload to a vendor. The harness makes the policy decision; <span className={mark}>Flowtel records what happened.</span> I am building it in the open, starting with Pi and a stable schema. If your agents work in real repositories, this is the piece I want to make <span className="font-semibold text-orange">boring, portable, and easy to trust.</span></p></div>
        </section>
      </div>
    </main>
  );
}
