"use client";

import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";

export default function Apply() {
  const router = useRouter();

  return (
    <div className="flex flex-col items-center justify-center h-screen bg-black text-white">
      {/* Corner brackets */}
      {/* Top Left */}
      <div className="absolute top-8 left-8 w-16 h-16 border-l-2 border-t-2 border-amber-400" />
      {/* Top Right */}
      <div className="absolute top-8 right-8 w-16 h-16 border-r-2 border-t-2 border-amber-400" />
      {/* Bottom Left */}
      <div className="absolute bottom-8 left-8 w-16 h-16 border-l-2 border-b-2 border-amber-400" />
      {/* Bottom Right */}
      <div className="absolute bottom-8 right-8 w-16 h-16 border-r-2 border-b-2 border-amber-400" />

      <h1>Under Construction</h1>
      <p>Please check back later</p>
      <Button onClick={() => router.push("/")}>Go Back</Button>
    </div>
  );
}