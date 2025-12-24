import { NextRequest, NextResponse } from "next/server";

export async function POST(request: NextRequest) {
  const apiUrl = process.env.API_URL || "http://localhost:8090";
  
  try {
    const body = await request.json();
    
    // Forward cookies from the client request to the backend
    const cookieHeader = request.headers.get("cookie");
    
    const response = await fetch(`${apiUrl}/api/login`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(cookieHeader && { Cookie: cookieHeader }),
      },
      body: JSON.stringify(body),
    });

    const data = await response.json();

    // Create response and forward any cookies from the backend
    const nextResponse = NextResponse.json(data, { status: response.status });
    
    // Forward Set-Cookie headers from backend
    const setCookie = response.headers.get("set-cookie");
    if (setCookie) {
      nextResponse.headers.set("set-cookie", setCookie);
    }

    return nextResponse;
  } catch (error) {
    console.error("Login proxy error:", error);
    return NextResponse.json(
      { error: "Failed to connect to server" },
      { status: 500 }
    );
  }
}

