import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone", // Required for Docker deployment
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        // Use Docker service name instead of localhost
        destination: `${process.env.API_URL || "http://localhost:8090"}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
