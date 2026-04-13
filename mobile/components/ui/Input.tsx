import { useState } from "react";
import { View, Text, TextInput, TextInputProps } from "react-native";

interface InputProps extends TextInputProps {
  label?: string;
  error?: string;
  testID?: string;
}

export default function Input({ label, error, onFocus, onBlur, ...props }: InputProps) {
  const [isFocused, setIsFocused] = useState(false);

  const borderClass = error
    ? "border-red-500"
    : isFocused
      ? "border-primary-500"
      : "border-gray-300";

  return (
    <View className="w-full mb-4">
      {label && <Text className="text-sm font-medium text-gray-700 mb-1">{label}</Text>}
      <TextInput
        className={`w-full border ${borderClass} rounded-xl px-4 py-3 text-base`}
        placeholderTextColor="#9ca3af"
        onFocus={(e) => {
          setIsFocused(true);
          onFocus?.(e);
        }}
        onBlur={(e) => {
          setIsFocused(false);
          onBlur?.(e);
        }}
        {...props}
      />
      {error && <Text className="text-sm text-red-500 mt-1">{error}</Text>}
    </View>
  );
}
