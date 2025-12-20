"use client";

import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";

export default function Home() {
  const router = useRouter();

  return (
    <div className="bg-black text-white flex flex-col items-center justify-center h-screen relative overflow-hidden">
      {/* Corner brackets */}
      {/* Top Left */}
      <div className="absolute top-8 left-8 w-16 h-16 border-l-2 border-t-2 border-amber-400" />
      {/* Top Right */}
      <div className="absolute top-8 right-8 w-16 h-16 border-r-2 border-t-2 border-amber-400" />
      {/* Bottom Left */}
      <div className="absolute bottom-8 left-8 w-16 h-16 border-l-2 border-b-2 border-amber-400" />
      {/* Bottom Right */}
      <div className="absolute bottom-8 right-8 w-16 h-16 border-r-2 border-b-2 border-amber-400" />

      <h1 className="text-6xl py-24 font-bold bg-linear-to-b from-gray-200 to-gray-500 bg-clip-text text-transparent tracking-wide">CIN CIN</h1>

      <div className="flex flex-row items-center gap-4 w-full max-w-md">
        <div className="flex-1 h-px bg-amber-400"></div>
        <p className="text-gray-300 text-lg font-thin tracking-widest">EXCLUSIVE MEMBERSHIP</p>
        <div className="flex-1 h-px bg-amber-400"></div>
      </div>

      <div className="flex flex-col items-stretch w-64 mt-8 gap-4">
        <Button className="bg-amber-400 text-black hover:bg-amber-500 cursor-pointer" onClick={() => router.push("/apply")}>APPLY FOR MEMBERSHIP</Button>
        <Button className="bg-amber-400 text-black hover:bg-amber-500 cursor-pointer" onClick={() => router.push("/login")}>LOGIN</Button>
      </div>

    </div>
  );
}