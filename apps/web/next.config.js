/** @type {import('next').NextConfig} */
const nextConfig = {
  experimental: {
    serverActions: { allowedOrigins: ["localhost:3000", "app.blutexts.com"] },
  },
  async rewrites() {
    return [
      // Proxy /api/* to the Go API in development
      {
        source: "/api/:path*",
        destination: `${process.env.API_URL || "http://localhost:8080"}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
