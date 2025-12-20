"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// TODO: Add validation to the form
// TODO: Add a loading state
// TODO: Add a success state
// TODO: Add a error state
// TODO: Add a forgot password button
// TODO: Add a remember me checkbox
// TODO: Add a terms of service checkbox
// TODO: Add a privacy policy checkbox
// TODO: Add a captcha
// TODO: Add a reCAPTCHA
// TODO: Add a cookie consent checkbox
// TODO: Add apply logic (have users fill out form and credit card info)
// TODO: Add Stripe

export default function LoginPage() {
    const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      const res = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      const data = await res.json();

      if (data.error) {
        setError(data.error);
      } else if (data.auth_token) {
        // Login successful, redirect to home
        router.push("/home");
      } else {
        setError("Unexpected response from server");
      }
    } catch (e) {
      setError("Failed to connect to server");
    } finally {
      setIsLoading(false);
    }
  };

    return (
        <div className="flex flex-col items-center justify-center h-screen bg-black text-white">
            {/* Corner brackets */}
            <div className="absolute top-8 left-8 w-16 h-16 border-l-2 border-t-2 border-amber-400" />
            <div className="absolute top-8 right-8 w-16 h-16 border-r-2 border-t-2 border-amber-400" />
            <div className="absolute bottom-8 left-8 w-16 h-16 border-l-2 border-b-2 border-amber-400" />
            <div className="absolute bottom-8 right-8 w-16 h-16 border-r-2 border-b-2 border-amber-400" />

      <h1 className="text-6xl py-8 font-bold bg-linear-to-b from-gray-200 to-gray-500 bg-clip-text text-transparent tracking-wide">
        LOGIN
      </h1>

      <p className="text-zinc-400 text-sm mb-8">Sign in with your Resy credentials</p>

      {error && (
        <div className="mb-4 p-4 bg-red-500/10 border border-red-500/30 rounded-lg text-red-400 max-w-xs text-center text-sm">
          {error}
        </div>
      )}

      <form onSubmit={handleLogin} className="flex flex-col items-center justify-center gap-4 w-full max-w-xs">
        <Input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className="bg-zinc-900 border-zinc-700 text-white placeholder:text-zinc-500"
        />
        <Input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          className="bg-zinc-900 border-zinc-700 text-white placeholder:text-zinc-500"
        />
        <Button
          type="submit"
          disabled={isLoading}
          className="w-full bg-amber-400 text-black mt-4 hover:bg-amber-500 cursor-pointer disabled:opacity-50"
        >
          {isLoading ? "SIGNING IN..." : "ENTER"}
        </Button>
      </form>

      <button
        onClick={() => router.push("/")}
        className="mt-8 text-zinc-500 text-sm hover:text-zinc-300 transition-colors"
      >
        ← Back to home
      </button>
        </div>
    );
}
