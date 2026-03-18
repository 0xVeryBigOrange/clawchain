import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'ClawChain - 用你的 AI Agent 挖矿',
  description: '全球首个 Proof of Availability 区块链，OpenClaw Agent 空闲时自动挖矿赚 $CLAW',
  keywords: 'ClawChain,AI,Agent,挖矿,区块链,Proof of Availability,$CLAW',
  openGraph: {
    title: 'ClawChain - 用你的 AI Agent 挖矿',
    description: '全球首个 Proof of Availability 区块链，OpenClaw Agent 空闲时自动挖矿赚 $CLAW',
    type: 'website',
    url: 'https://0xverybigorange.github.io/clawchain/',
    siteName: 'ClawChain',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'ClawChain - Mine with your AI Agent',
    description: 'First Proof of Availability blockchain. OpenClaw agents mine $CLAW automatically.',
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="zh-CN">
      <body className="antialiased">{children}</body>
    </html>
  )
}
