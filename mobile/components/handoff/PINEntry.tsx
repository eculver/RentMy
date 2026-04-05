/**
 * PINEntry — 4-digit PIN entry for the renter at check-in.
 *
 * Auto-advances focus to the next digit on input and auto-submits when all
 * four digits are filled.
 */
import { useRef, useState } from "react";
import {
  View,
  Text,
  TextInput,
  Pressable,
  ActivityIndicator,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";

interface PINEntryProps {
  verified: boolean;
  verifying: boolean;
  error: string | null;
  onSubmit: (pin: string) => void;
}

const PIN_LENGTH = 4;

export default function PINEntry({
  verified,
  verifying,
  error,
  onSubmit,
}: PINEntryProps) {
  const [digits, setDigits] = useState<string[]>(Array(PIN_LENGTH).fill(""));
  const inputRefs = useRef<(TextInput | null)[]>([]);

  const handleChange = (index: number, value: string) => {
    // Only accept a single digit
    const digit = value.replace(/[^0-9]/g, "").slice(-1);
    const next = [...digits];
    next[index] = digit;
    setDigits(next);

    if (digit && index < PIN_LENGTH - 1) {
      inputRefs.current[index + 1]?.focus();
    }

    if (next.every((d) => d !== "")) {
      onSubmit(next.join(""));
    }
  };

  const handleKeyPress = (index: number, key: string) => {
    if (key === "Backspace" && !digits[index] && index > 0) {
      inputRefs.current[index - 1]?.focus();
    }
  };

  const pin = digits.join("");
  const canSubmit = pin.length === PIN_LENGTH && !verifying && !verified;

  return (
    <View className="bg-gray-50 rounded-2xl px-4 py-4 gap-y-3">
      {/* Header */}
      <View className="flex-row items-center gap-x-3">
        <View
          className={`w-10 h-10 rounded-full items-center justify-center ${
            verified ? "bg-green-100" : error ? "bg-red-100" : "bg-blue-50"
          }`}
        >
          {verifying ? (
            <ActivityIndicator size="small" color="#0284c7" />
          ) : verified ? (
            <Ionicons name="checkmark-circle" size={24} color="#16a34a" />
          ) : (
            <Ionicons name="keypad-outline" size={22} color="#0284c7" />
          )}
        </View>

        <View className="flex-1">
          <Text className="text-sm font-semibold text-gray-900">
            {verified ? "PIN verified" : "Enter check-in PIN"}
          </Text>
          <Text className="text-xs text-gray-500 mt-0.5">
            {verified
              ? "PIN accepted — check-in confirmed."
              : "Ask the host for the 4-digit PIN shown on their screen."}
          </Text>
        </View>
      </View>

      {/* Digit inputs */}
      {!verified && (
        <View className="flex-row justify-center gap-x-3">
          {Array.from({ length: PIN_LENGTH }).map((_, i) => (
            <TextInput
              key={i}
              ref={(ref) => {
                inputRefs.current[i] = ref;
              }}
              className="w-14 h-14 border-2 border-gray-300 rounded-xl text-center text-2xl font-bold text-gray-900 bg-white"
              style={{ borderColor: digits[i] ? "#0284c7" : "#d1d5db" }}
              keyboardType="number-pad"
              maxLength={1}
              value={digits[i]}
              onChangeText={(v) => handleChange(i, v)}
              onKeyPress={({ nativeEvent }) =>
                handleKeyPress(i, nativeEvent.key)
              }
              editable={!verifying && !verified}
              selectTextOnFocus
            />
          ))}
        </View>
      )}

      {/* Error notice */}
      {error && !verified && (
        <View className="bg-red-50 rounded-xl px-3 py-2">
          <Text className="text-xs text-red-700">{error}</Text>
        </View>
      )}

      {/* Manual submit (fallback if auto-submit didn't fire) */}
      {!verified && (
        <Pressable
          onPress={() => canSubmit && onSubmit(pin)}
          disabled={!canSubmit}
          className={`rounded-xl py-3 items-center ${
            canSubmit ? "bg-sky-600" : "bg-gray-200"
          }`}
        >
          <Text
            className={`text-sm font-semibold ${
              canSubmit ? "text-white" : "text-gray-400"
            }`}
          >
            {verifying ? "Checking…" : "Confirm PIN"}
          </Text>
        </Pressable>
      )}
    </View>
  );
}
