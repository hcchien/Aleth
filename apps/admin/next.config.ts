import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/admin/:path*",
        destination: `${process.env.ADMIN_API_URL ?? "http://localhost:8087"}/:path*`,
      },
    ];
  },
};

export default nextConfig;
