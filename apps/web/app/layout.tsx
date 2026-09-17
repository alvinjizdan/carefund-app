import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "CareFund — Transparent Medical & Social Crowdfunding",
  description: "CareFund is a verified crowdfunding platform ensuring financial integrity and impact transparency.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased min-h-screen flex flex-col font-sans">
        {children}
      </body>
    </html>
  );
}
