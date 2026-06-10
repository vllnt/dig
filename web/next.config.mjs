import createMDX from '@next/mdx'
import createNextIntlPlugin from 'next-intl/plugin'

const withNextIntl = createNextIntlPlugin('./i18n/request.ts')

const withMDX = createMDX({
  options: {
    remarkPlugins: [],
    rehypePlugins: [],
  },
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  pageExtensions: ['js', 'jsx', 'md', 'mdx', 'ts', 'tsx'],
  transpilePackages: ['@vllnt/ui', '@vllnt/analytics'],
  experimental: {
    optimizePackageImports: ['@vllnt/ui', '@vllnt/analytics'],
  },
}

export default withNextIntl(withMDX(nextConfig))
