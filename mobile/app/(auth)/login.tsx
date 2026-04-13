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
  email: z.string().email("Enter a valid email address"),
  password: z.string().min(8, "Password must be at least 8 characters"),
});

type FormData = z.infer<typeof schema>;

export default function LoginScreen() {
  const loginWithCredentials = useAuthStore((s) => s.loginWithCredentials);
  const [apiError, setApiError] = useState<string | null>(null);

  const {
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { email: "", password: "" },
  });

  const onSubmit = async (data: FormData) => {
    setApiError(null);
    try {
      await loginWithCredentials(data.email, data.password);
    } catch (err) {
      if (err instanceof HTTPError && err.response.status === 401) {
        setApiError("Invalid email or password.");
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
        <View className="flex-1 items-center justify-center bg-white px-6 py-12" testID="screen-login">
          <Text className="text-3xl font-bold mb-2">RentMy</Text>
          <Text className="text-gray-500 mb-8">Rent anything nearby, fast</Text>

          <View className="w-full">
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
                  testID="input-email"
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
                  autoComplete="current-password"
                  testID="input-password"
                  value={value}
                  onChangeText={onChange}
                  onBlur={onBlur}
                  error={errors.password?.message}
                />
              )}
            />

            {apiError && (
              <Text testID="error-message" className="text-red-500 text-sm mb-3 text-center">{apiError}</Text>
            )}

            <Button
              title="Sign In"
              onPress={handleSubmit(onSubmit)}
              loading={isSubmitting}
              testID="btn-sign-in"
            />
          </View>

          <View className="mt-4">
            <Button
              title="Create an account"
              variant="ghost"
              testID="btn-create-account"
              onPress={() => router.push("/register")}
            />
          </View>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
