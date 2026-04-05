/**
 * PINDisplay — host-side check-in PIN display and SMS fallback trigger.
 *
 * The check-in PIN is generated server-side when the host accepts the booking
 * and is NOT returned through the proximity status API (it is stored securely
 * server-side). The host triggers an SMS delivery so the renter receives the
 * PIN on their device, or shows the PIN from a server-supplied prop if a
 * dedicated PIN-reveal endpoint is added in a future backend task.
 */
import { useState } from "react";
import {
  View,
  Text,
  Pressable,
  TextInput,
  ActivityIndicator,
  Alert,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { api } from "../../lib/api";

interface PINDisplayProps {
  transactionId: string;
  /** PIN value if available from a server-side reveal endpoint (optional). */
  pin?: string;
}

export default function PINDisplay({ transactionId, pin }: PINDisplayProps) {
  const [phone, setPhone] = useState("");
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);

  const handleSendSMS = async () => {
    if (!phone.trim()) {
      Alert.alert("Phone required", "Enter the renter's phone number (E.164 format, e.g. +12135551234).");
      return;
    }

    setSending(true);
    try {
      await api.post("api/v1/proximity/sms-fallback", {
        json: { transactionId, toPhone: phone.trim() },
      });
      setSent(true);
    } catch {
      Alert.alert("Failed to send", "Could not send the PIN via SMS. Please try again.");
    } finally {
      setSending(false);
    }
  };

  return (
    <View className="bg-gray-50 rounded-2xl px-4 py-4 gap-y-3">
      {/* Header */}
      <View className="flex-row items-center gap-x-3">
        <View className="w-10 h-10 rounded-full bg-blue-50 items-center justify-center">
          <Ionicons name="keypad-outline" size={22} color="#0284c7" />
        </View>
        <View className="flex-1">
          <Text className="text-sm font-semibold text-gray-900">
            Check-in PIN
          </Text>
          <Text className="text-xs text-gray-500 mt-0.5">
            The renter needs this PIN to complete check-in.
          </Text>
        </View>
      </View>

      {/* PIN large display (if available) */}
      {pin ? (
        <View className="bg-white border border-gray-200 rounded-xl py-4 items-center">
          <Text className="text-4xl font-bold tracking-[0.4em] text-gray-900 font-mono">
            {pin}
          </Text>
          <Text className="text-xs text-gray-400 mt-2">Show this to the renter</Text>
        </View>
      ) : (
        <View className="bg-blue-50 rounded-xl px-3 py-2">
          <Text className="text-xs text-blue-700">
            The PIN was generated when you accepted the booking. Send it to the renter via SMS below.
          </Text>
        </View>
      )}

      {/* SMS fallback */}
      {sent ? (
        <View className="flex-row items-center gap-x-2 bg-green-50 rounded-xl px-3 py-2">
          <Ionicons name="checkmark-circle" size={16} color="#16a34a" />
          <Text className="text-xs text-green-700 flex-1">
            PIN sent via SMS to {phone}.
          </Text>
        </View>
      ) : (
        <View className="gap-y-2">
          <Text className="text-xs font-medium text-gray-600">
            Send PIN via SMS to renter
          </Text>
          <TextInput
            className="border border-gray-300 rounded-xl px-3 py-2.5 text-sm text-gray-900 bg-white"
            placeholder="+12135551234"
            placeholderTextColor="#9ca3af"
            value={phone}
            onChangeText={setPhone}
            keyboardType="phone-pad"
            autoComplete="tel"
          />
          <Pressable
            onPress={handleSendSMS}
            disabled={sending || !phone.trim()}
            className={`rounded-xl py-3 items-center flex-row justify-center gap-x-2 ${
              sending || !phone.trim() ? "bg-gray-200" : "bg-sky-600"
            }`}
          >
            {sending ? (
              <ActivityIndicator size="small" color="white" />
            ) : (
              <Ionicons name="send-outline" size={16} color="white" />
            )}
            <Text
              className={`text-sm font-semibold ${
                sending || !phone.trim() ? "text-gray-400" : "text-white"
              }`}
            >
              {sending ? "Sending…" : "Send PIN"}
            </Text>
          </Pressable>
        </View>
      )}
    </View>
  );
}
