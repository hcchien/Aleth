import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./i18n/request.ts");

const nextConfig: NextConfig = {
  async rewrites() {
    const gatewayUrl = process.env.GATEWAY_URL || "http://localhost:4000";
    const notificationUrl = process.env.NOTIFICATION_URL || "http://localhost:8084";
    return [
      {
        source: "/graphql",
        destination: `${gatewayUrl}/graphql`,
      },
      {
        source: "/api/notifications/:path*",
        destination: `${notificationUrl}/notifications/:path*`,
      },
    ];
  },
};

export default withNextIntl(nextConfig);
