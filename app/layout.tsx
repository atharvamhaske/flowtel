import "./globals.css";

export const metadata = {
  title: "Flowtel — See how agents act",
  description: "Vendor-neutral observability and governance for coding harnesses.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
