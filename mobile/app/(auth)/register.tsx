import { useState } from "react";
import { View, Text, ScrollView, KeyboardAvoidingView, Platform } from "react-native";
import { router } from "expo-router";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { HTTPError } from "ky";
import { useAuthStore } from "../../lib/auth";
import Input from "../../components/ui/Input";
import Button from "../../components/ui/Button";

const schema = z.object({
  name: z.string().min(1, "Name is required"),
  email: z.string().email("Enter a valid email address"),
  password: z
    .string()
    .min(8, "Password must be at least 8 characters")
    .regex(/[A-Z]/, "Password must contain at least one uppercase letter")
    .regex(/[0-9]/, "Password must contain at least one number"),
});

type FormData = z.infer<typeof schema>;

export default function RegisterScreen() {
  const register = useAuthStore((s) => s.register);
  const [apiError, setApiError] = useState<string | null>(null);

  const {
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", email: "", password: "" },
  });

  const onSubmit = async (data: FormData) => {
    setApiError(null);
    try {
      await register(data.name, data.email, data.password);
    } catch (err) {
      if (err instanceof HTTPError && err.response.status === 409) {
        setApiError("An account with this email already exists.");
      } else {
        setApiError("Something went wrong. Please try again.");
      }
    }
  };

  return (
    <KeyboardAvoidingView
      className="flex-1"
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView
        contentContainerStyle={{ flexGrow: 1 }}
        keyboardShouldPersistTaps="handled"
      >
        <View className="flex-1 items-center justify-center bg-white px-6 py-12">
          <Text className="text-2xl font-bold mb-2">Create Account</Text>
          <Text className="text-gray-500 mb-8">Join RentMy to start renting</Text>

          <View className="w-full">
            <Controller
              control={control}
              name="name"
              render={({ field: { value, onChange, onBlur } }) => (
                <Input
                  label="Full Name"
                  placeholder="Jane Smith"
                  autoCapitalize="words"
                  autoComplete="name"
                  value={value}
                  onChangeText={onChange}
                  onBlur={onBlur}
                  error={errors.name?.message}
                />
              )}
            />

            <Controller
              control={control}
              name="email"
              render={({ field: { value, onChange, onBlur } }) => (
                <Input
                  label="Email"
                  placeholder="you@example.com"
                  keyboardType="email-address"
                  autoCapitalize="none"
                  autoComplete="email"
                  value={value}
                  onChangeText={onChange}
                  onBlur={onBlur}
                  error={errors.email?.message}
                />
              )}
            />

            <Controller
              control={control}
              name="password"
              render={({ field: { value, onChange, onBlur } }) => (
                <Input
                  label="Password"
                  placeholder="••••••••"
                  secureTextEntry
                  autoComplete="new-password"
                  value={value}
                  onChangeText={onChange}
                  onBlur={onBlur}
                  error={errors.password?.message}
                />
              )}
            />

            {apiError && (
              <Text className="text-red-500 text-sm mb-3 text-center">{apiError}</Text>
            )}

            <Button
              title="Create Account"
              onPress={handleSubmit(onSubmit)}
              loading={isSubmitting}
            />
          </View>

          <View className="mt-4">
            <Button
              title="Already have an account? Sign in"
              variant="ghost"
              onPress={() => router.back()}
            />
          </View>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
