"use client";

import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";

const RESTAURANTS = [
  { venue_id: 89607, name: "Crevette" },
  { venue_id: 53251, name: "Opera House" },
  { venue_id: 70599, name: "Forgione" },
];

const TABLE_TYPES = [
  { value: "dining", label: "Dining Room" },
  { value: "indoor", label: "Indoor" },
  { value: "outdoor", label: "Outdoor" },
  { value: "patio", label: "Patio" },
  { value: "bar", label: "Bar" },
  { value: "lounge", label: "Lounge" },
  { value: "booth", label: "Booth" },
];

export default function Home() {
  // Restaurant selection
  const [selectedVenueId, setSelectedVenueId] = useState<number | null>(null);

  // Reservation form state
  const [reservationDate, setReservationDate] = useState("");
  const [reservationTime, setReservationTime] = useState("");
  const [tablePreferences, setTablePreferences] = useState<string[]>([]);
  const [isImmediate, setIsImmediate] = useState(true);
  const [scheduledDate, setScheduledDate] = useState("");
  const [scheduledTime, setScheduledTime] = useState("");

  // UI state
  const [isReserving, setIsReserving] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const [logs, setLogs] = useState<string[]>([]);

  // Fetch logs periodically
  useEffect(() => {
    const fetchLogs = async () => {
      try {
        const res = await fetch("/api/logs");
        if (res.ok) {
          const data = await res.json();
          setLogs(data.slice(-10)); // Last 10 logs
        }
      } catch {
        // Ignore errors
      }
    };

    fetchLogs();
    const interval = setInterval(fetchLogs, 5000);
    return () => clearInterval(interval);
  }, []);

  const toggleTablePreference = (value: string) => {
    setTablePreferences((prev) =>
      prev.includes(value) ? prev.filter((p) => p !== value) : [...prev, value]
    );
  };

  const handleReserve = async () => {
    if (!selectedVenueId) {
      setMessage({ type: "error", text: "Please select a restaurant first" });
      return;
    }

    if (!reservationDate || !reservationTime) {
      setMessage({ type: "error", text: "Please select a date and time" });
      return;
    }

    if (!isImmediate && (!scheduledDate || !scheduledTime)) {
      setMessage({ type: "error", text: "Please set when to attempt the reservation" });
      return;
    }

    setIsReserving(true);
    setMessage(null);

    const reservationDateTime = `${reservationDate}T${reservationTime}`;
    const requestDateTime = isImmediate ? "" : `${scheduledDate}T${scheduledTime}`;

    try {
      const res = await fetch("/api/reserve", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          venue_id: selectedVenueId,
          reservation_time: reservationDateTime,
          party_size: 2,
          table_preferences: tablePreferences,
          is_immediate: isImmediate,
          request_time: requestDateTime,
        }),
      });

      const data = await res.json();

      if (data.error) {
        setMessage({ type: "error", text: data.error });
      } else if (data.reservation_time) {
        setMessage({ type: "success", text: `Reserved for ${data.reservation_time}` });
      } else if (data.reservation_id) {
        setMessage({ type: "success", text: `Scheduled! ID: ${data.reservation_id}` });
      }
    } catch {
      setMessage({ type: "error", text: "Failed to make reservation" });
    } finally {
      setIsReserving(false);
    }
  };

  return (
    <div className="min-h-screen bg-black text-white relative overflow-hidden">
      {/* Corner brackets */}
      <div className="absolute top-8 left-8 w-16 h-16 border-l-2 border-t-2 border-amber-400" />
      <div className="absolute top-8 right-8 w-16 h-16 border-r-2 border-t-2 border-amber-400" />
      <div className="absolute bottom-8 left-8 w-16 h-16 border-l-2 border-b-2 border-amber-400" />
      <div className="absolute bottom-8 right-8 w-16 h-16 border-r-2 border-b-2 border-amber-400" />

      {/* Header */}
      <div className="text-center pt-8 pb-4">
        <h1 className="text-4xl font-bold bg-linear-to-b from-gray-200 to-gray-500 bg-clip-text text-transparent tracking-wide">
          CIN CIN
        </h1>
      </div>

      {/* Main content */}
      <div className="max-w-xl mx-auto px-8 pb-24">
        <Card className="bg-zinc-900/50 border-zinc-800">
          <CardHeader>
            <CardTitle className="text-amber-400 text-2xl font-light tracking-wider text-center">
              RESERVE NOW
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Message */}
            {message && (
              <div
                className={`p-4 rounded-lg text-center ${
                  message.type === "success"
                    ? "bg-green-500/10 border border-green-500/30 text-green-400"
                    : "bg-red-500/10 border border-red-500/30 text-red-400"
                }`}
              >
                {message.text}
              </div>
            )}

            {/* Restaurant Selection */}
            <div className="space-y-2">
              <Label className="text-zinc-400 text-sm">Restaurant</Label>
              <div className="grid grid-cols-3 gap-2">
                {RESTAURANTS.map((restaurant) => (
                  <button
                    key={restaurant.venue_id}
                    onClick={() => setSelectedVenueId(restaurant.venue_id)}
                    className={`py-3 px-2 rounded-lg text-sm font-medium transition-colors ${
                      selectedVenueId === restaurant.venue_id
                        ? "bg-amber-400 text-black"
                        : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                    }`}
                  >
                    {restaurant.name}
                  </button>
                ))}
              </div>
            </div>

            {/* Date & Time */}
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label className="text-zinc-400 text-sm">Reservation Date</Label>
                <Input
                  type="date"
                  value={reservationDate}
                  onChange={(e) => setReservationDate(e.target.value)}
                  className="bg-zinc-800 border-zinc-700 text-white"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-zinc-400 text-sm">Reservation Time</Label>
                <Input
                  type="time"
                  value={reservationTime}
                  onChange={(e) => setReservationTime(e.target.value)}
                  className="bg-zinc-800 border-zinc-700 text-white"
                />
              </div>
            </div>

            {/* Party Size (hardcoded display) */}
            <div className="flex items-center justify-between p-3 bg-zinc-800/50 rounded-lg">
              <span className="text-zinc-400 text-sm">Party Size</span>
              <span className="text-white font-medium">2 guests</span>
            </div>

            {/* Table Preferences */}
            <div className="space-y-2">
              <Label className="text-zinc-400 text-sm">Table Preferences (Optional)</Label>
              <div className="flex flex-wrap gap-2">
                {TABLE_TYPES.map((type) => (
                  <button
                    key={type.value}
                    onClick={() => toggleTablePreference(type.value)}
                    className={`px-3 py-1.5 rounded-full text-sm transition-colors ${
                      tablePreferences.includes(type.value)
                        ? "bg-amber-400 text-black"
                        : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                    }`}
                  >
                    {type.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Timing Toggle */}
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setIsImmediate(true)}
                  className={`flex-1 py-3 rounded-lg text-sm font-medium transition-colors ${
                    isImmediate
                      ? "bg-amber-400 text-black"
                      : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                  }`}
                >
                  Reserve Immediately
                </button>
                <button
                  onClick={() => setIsImmediate(false)}
                  className={`flex-1 py-3 rounded-lg text-sm font-medium transition-colors ${
                    !isImmediate
                      ? "bg-amber-400 text-black"
                      : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                  }`}
                >
                  Schedule for Later
                </button>
              </div>

              {/* Scheduled Time (if not immediate) */}
              {!isImmediate && (
                <div className="grid grid-cols-2 gap-4 p-4 bg-zinc-800/50 rounded-lg">
                  <div className="space-y-2">
                    <Label className="text-zinc-400 text-sm">Attempt On Date</Label>
                    <Input
                      type="date"
                      value={scheduledDate}
                      onChange={(e) => setScheduledDate(e.target.value)}
                      className="bg-zinc-700 border-zinc-600 text-white"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-zinc-400 text-sm">Attempt At Time</Label>
                    <Input
                      type="time"
                      value={scheduledTime}
                      onChange={(e) => setScheduledTime(e.target.value)}
                      className="bg-zinc-700 border-zinc-600 text-white"
                    />
                  </div>
                  <p className="col-span-full text-xs text-zinc-500">
                    The bot will attempt to book exactly at this time (NYC timezone)
                  </p>
                </div>
              )}
            </div>

            {/* Reserve Button */}
            <Button
              onClick={handleReserve}
              disabled={isReserving || !selectedVenueId}
              className="w-full py-6 text-lg bg-amber-400 text-black hover:bg-amber-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isReserving ? "RESERVING..." : isImmediate ? "RESERVE NOW" : "SCHEDULE RESERVATION"}
            </Button>

            {!selectedVenueId && (
              <p className="text-center text-zinc-500 text-sm">
                Select a restaurant to continue
              </p>
            )}
          </CardContent>
        </Card>

        {/* Activity Log */}
        <Card className="bg-zinc-900/50 border-zinc-800 mt-6">
          <CardHeader className="pb-3">
            <CardTitle className="text-amber-400 text-lg font-light tracking-wider">
              ACTIVITY
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2 max-h-32 overflow-y-auto text-xs font-mono">
              {logs.length === 0 ? (
                <p className="text-zinc-500">No recent activity</p>
              ) : (
                logs.map((log, i) => (
                  <p key={i} className="text-zinc-400 wrap-break-word">
                    {log}
                  </p>
                ))
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
