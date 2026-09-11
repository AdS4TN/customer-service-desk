import type { Metadata } from "next"
import { Geist, Geist_Mono } from "next/font/google"

import { AuthProvider } from "@/components/auth-provider"
import { ThemeProvider } from "@/components/theme-provider"
import { Toaster } from "@/components/ui/sonner"
import { AppI18nProvider } from "@/i18n/provider"

import "../(dashboard)/dashboard.css"
import "@/styles/main.scss"

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
})

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
})

const paletteScript = `
try {
  var palette = window.localStorage.getItem("dashboard_palette");
  document.documentElement.dataset.palette = palette === "plain" || palette === "blue" || palette === "green" || palette === "gray" ? palette : "plain";
} catch (_) {
  document.documentElement.dataset.palette = "plain";
}
`

export const metadata: Metadata = {
  title: "AI Customer Service Admin",
  description: "AI Customer Service Admin",
}

export default function AuthRootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en-US" className={`${geistSans.variable} ${geistMono.variable}`} suppressHydrationWarning>
      <body className="antialiased font-sans">
        <script dangerouslySetInnerHTML={{ __html: paletteScript }} />
        <AppI18nProvider>
          <ThemeProvider>
            <AuthProvider>
              {children}
              <Toaster position="top-center" richColors />
            </AuthProvider>
          </ThemeProvider>
        </AppI18nProvider>
      </body>
    </html>
  )
}
