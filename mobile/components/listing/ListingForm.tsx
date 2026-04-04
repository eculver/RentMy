import { useState } from "react";
import {
  View,
  Text,
  Pressable,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import Input from "../ui/Input";
import Button from "../ui/Button";

const DURATION_OPTIONS = [
  { label: "1 day", value: "24h" },
  { label: "2 days", value: "48h" },
  { label: "3 days", value: "72h" },
  { label: "7 days (max)", value: "168h" },
] as const;

const schema = z.object({
  title: z.string().min(3, "Title must be at least 3 characters"),
  description: z.string().min(10, "Description must be at least 10 characters"),
  pricePerDay: z.coerce
    .number()
    .positive("Must be a positive number"),
  pricePerHour: z.coerce
    .number()
    .positive("Must be a positive number")
    .optional()
    .or(z.literal("")),
  maxDuration: z.string().min(1, "Select a max rental duration"),
  lat: z.coerce
    .number()
    .min(-90, "Latitude must be between -90 and 90")
    .max(90, "Latitude must be between -90 and 90"),
  lng: z.coerce
    .number()
    .min(-180, "Longitude must be between -180 and 180")
    .max(180, "Longitude must be between -180 and 180"),
});

export type ListingFormData = {
  title: string;
  description: string;
  pricePerDay: number;
  pricePerHour?: number;
  maxDuration: string;
  lat: number;
  lng: number;
};

interface ListingFormProps {
  onSubmit: (data: ListingFormData) => Promise<void>;
}

export default function ListingForm({ onSubmit }: ListingFormProps) {
  const [selectedDuration, setSelectedDuration] = useState("24h");

  const {
    control,
    handleSubmit,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      title: "",
      description: "",
      pricePerDay: "" as unknown as number,
      pricePerHour: "" as unknown as number | undefined,
      maxDuration: "24h",
      lat: "" as unknown as number,
      lng: "" as unknown as number,
    },
  });

  const handleDurationSelect = (value: string) => {
    setSelectedDuration(value);
    setValue("maxDuration", value, { shouldValidate: true });
  };

  const onFormSubmit = handleSubmit(async (data) => {
    const payload: ListingFormData = {
      title: data.title,
      description: data.description,
      pricePerDay: Number(data.pricePerDay),
      maxDuration: data.maxDuration,
      lat: Number(data.lat),
      lng: Number(data.lng),
    };
    if (data.pricePerHour && Number(data.pricePerHour) > 0) {
      payload.pricePerHour = Number(data.pricePerHour);
    }
    await onSubmit(payload);
  });

  return (
    <KeyboardAvoidingView
      className="flex-1"
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView
        contentContainerStyle={{ paddingBottom: 40 }}
        keyboardShouldPersistTaps="handled"
      >
        <View className="px-6 pt-6">
          <Text className="text-xl font-bold mb-6">Listing Details</Text>

          <Controller
            control={control}
            name="title"
            render={({ field: { value, onChange, onBlur } }) => (
              <Input
                label="Title"
                placeholder="e.g. Ocean kayak, DSLR camera, power drill"
                value={value}
                onChangeText={onChange}
                onBlur={onBlur}
                error={errors.title?.message}
              />
            )}
          />

          <Controller
            control={control}
            name="description"
            render={({ field: { value, onChange, onBlur } }) => (
              <Input
                label="Description"
                placeholder="Describe your item, condition, and any special notes"
                value={value}
                onChangeText={onChange}
                onBlur={onBlur}
                error={errors.description?.message}
                multiline
                numberOfLines={3}
              />
            )}
          />

          <Text className="text-sm font-medium text-gray-700 mb-1">Pricing</Text>
          <View className="flex-row gap-3 mb-4">
            <View className="flex-1">
              <Controller
                control={control}
                name="pricePerDay"
                render={({ field: { value, onChange, onBlur } }) => (
                  <Input
                    label="Per day ($)"
                    placeholder="25"
                    keyboardType="decimal-pad"
                    value={value != null && value !== ("" as unknown as number) ? String(value) : ""}
                    onChangeText={onChange}
                    onBlur={onBlur}
                    error={errors.pricePerDay?.message}
                  />
                )}
              />
            </View>
            <View className="flex-1">
              <Controller
                control={control}
                name="pricePerHour"
                render={({ field: { value, onChange, onBlur } }) => (
                  <Input
                    label="Per hour ($ optional)"
                    placeholder="5"
                    keyboardType="decimal-pad"
                    value={value != null && value !== ("" as unknown as number) ? String(value) : ""}
                    onChangeText={onChange}
                    onBlur={onBlur}
                    error={errors.pricePerHour?.message}
                  />
                )}
              />
            </View>
          </View>

          <Text className="text-sm font-medium text-gray-700 mb-2">
            Max rental duration
          </Text>
          <View className="flex-row flex-wrap gap-2 mb-4">
            {DURATION_OPTIONS.map((opt) => (
              <Pressable
                key={opt.value}
                onPress={() => handleDurationSelect(opt.value)}
                className={`px-4 py-2 rounded-xl border ${
                  selectedDuration === opt.value
                    ? "bg-sky-600 border-sky-600"
                    : "bg-white border-gray-300"
                }`}
              >
                <Text
                  className={`text-sm font-medium ${
                    selectedDuration === opt.value
                      ? "text-white"
                      : "text-gray-700"
                  }`}
                >
                  {opt.label}
                </Text>
              </Pressable>
            ))}
          </View>
          {errors.maxDuration && (
            <Text className="text-red-500 text-sm mb-3">
              {errors.maxDuration.message}
            </Text>
          )}

          <Text className="text-sm font-medium text-gray-700 mb-1">
            Location (lat / lng)
          </Text>
          <View className="flex-row gap-3 mb-4">
            <View className="flex-1">
              <Controller
                control={control}
                name="lat"
                render={({ field: { value, onChange, onBlur } }) => (
                  <Input
                    label="Latitude"
                    placeholder="33.77"
                    keyboardType="decimal-pad"
                    value={value != null && value !== ("" as unknown as number) ? String(value) : ""}
                    onChangeText={onChange}
                    onBlur={onBlur}
                    error={errors.lat?.message}
                  />
                )}
              />
            </View>
            <View className="flex-1">
              <Controller
                control={control}
                name="lng"
                render={({ field: { value, onChange, onBlur } }) => (
                  <Input
                    label="Longitude"
                    placeholder="-118.19"
                    keyboardType="decimal-pad"
                    value={value != null && value !== ("" as unknown as number) ? String(value) : ""}
                    onChangeText={onChange}
                    onBlur={onBlur}
                    error={errors.lng?.message}
                  />
                )}
              />
            </View>
          </View>

          <Button
            title="Create Listing"
            onPress={onFormSubmit}
            loading={isSubmitting}
          />
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
