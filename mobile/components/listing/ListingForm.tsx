import { useEffect, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ScrollView,
  KeyboardAvoidingView,
  Keyboard,
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

export interface AISuggestions {
  title?: string;
  description?: string;
  pricePerDay?: number;
  pricePerHour?: number;
  tags?: string[];
  /** The AI's estimated item value in cents, used for override threshold checks. */
  estimatedValueCents?: number;
}

interface ListingFormProps {
  onSubmit: (data: ListingFormData) => Promise<void>;
  aiSuggestions?: AISuggestions | null;
  /** Called when the user changes pricePerDay; provides the new value in dollars. */
  onPricePerDayChange?: (dollars: number) => void;
}

function AIBadge() {
  return (
    <View className="ml-1 px-1 py-0.5 bg-sky-100 rounded">
      <Text className="text-sky-600 text-[10px] font-semibold">AI</Text>
    </View>
  );
}

export default function ListingForm({
  onSubmit,
  aiSuggestions,
  onPricePerDayChange,
}: ListingFormProps) {
  const [selectedDuration, setSelectedDuration] = useState("24h");
  // Track which fields have been pre-filled by AI and not yet edited by the host
  const [aiFilledFields, setAiFilledFields] = useState<Set<string>>(new Set());

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

  // Apply AI suggestions when they arrive
  useEffect(() => {
    if (!aiSuggestions) return;
    const filled = new Set<string>();

    if (aiSuggestions.title) {
      setValue("title", aiSuggestions.title, { shouldValidate: false });
      filled.add("title");
    }
    if (aiSuggestions.description) {
      setValue("description", aiSuggestions.description, {
        shouldValidate: false,
      });
      filled.add("description");
    }
    if (aiSuggestions.pricePerDay) {
      setValue("pricePerDay", aiSuggestions.pricePerDay, {
        shouldValidate: false,
      });
      filled.add("pricePerDay");
    }
    if (aiSuggestions.pricePerHour) {
      setValue("pricePerHour", aiSuggestions.pricePerHour, {
        shouldValidate: false,
      });
      filled.add("pricePerHour");
    }

    setAiFilledFields(filled);
  }, [aiSuggestions, setValue]);

  const markEdited = (field: string) => {
    if (aiFilledFields.has(field)) {
      setAiFilledFields((prev) => {
        const next = new Set(prev);
        next.delete(field);
        return next;
      });
    }
  };

  const handleDurationSelect = (value: string) => {
    setSelectedDuration(value);
    setValue("maxDuration", value, { shouldValidate: true });
  };

  const onFormSubmit = handleSubmit(async (data) => {
    Keyboard.dismiss();
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
        keyboardDismissMode="on-drag"
      >
        <View className="px-6 pt-6">
          <Text className="text-xl font-bold mb-6">Listing Details</Text>

          {/* AI-suggested tags */}
          {aiSuggestions?.tags && aiSuggestions.tags.length > 0 && (
            <View className="mb-4">
              <View className="flex-row items-center mb-2">
                <Text className="text-sm font-medium text-gray-700">Tags</Text>
                <AIBadge />
              </View>
              <View className="flex-row flex-wrap gap-2">
                {aiSuggestions.tags.map((tag) => (
                  <View
                    key={tag}
                    className="px-3 py-1 bg-sky-50 border border-sky-200 rounded-full"
                  >
                    <Text className="text-xs text-sky-700">{tag}</Text>
                  </View>
                ))}
              </View>
            </View>
          )}

          <Controller
            control={control}
            name="title"
            render={({ field: { value, onChange, onBlur } }) => (
              <View>
                <View className="flex-row items-center mb-1">
                  <Text className="text-sm font-medium text-gray-700">
                    Title
                  </Text>
                  {aiFilledFields.has("title") && <AIBadge />}
                </View>
                <Input
                  testID="input-listing-title"
                  placeholder="e.g. Ocean kayak, DSLR camera, power drill"
                  value={value}
                  onChangeText={(text) => {
                    markEdited("title");
                    onChange(text);
                  }}
                  onBlur={onBlur}
                  error={errors.title?.message}
                />
              </View>
            )}
          />

          <Controller
            control={control}
            name="description"
            render={({ field: { value, onChange, onBlur } }) => (
              <View>
                <View className="flex-row items-center mb-1">
                  <Text className="text-sm font-medium text-gray-700">
                    Description
                  </Text>
                  {aiFilledFields.has("description") && <AIBadge />}
                </View>
                <Input
                  testID="input-listing-description"
                  placeholder="Describe your item, condition, and any special notes"
                  value={value}
                  onChangeText={(text) => {
                    markEdited("description");
                    onChange(text);
                  }}
                  onBlur={onBlur}
                  error={errors.description?.message}
                  multiline
                  numberOfLines={3}
                />
              </View>
            )}
          />

          <Text className="text-sm font-medium text-gray-700 mb-1">
            Pricing
          </Text>
          <View className="flex-row gap-3 mb-4">
            <View className="flex-1">
              <Controller
                control={control}
                name="pricePerDay"
                render={({ field: { value, onChange, onBlur } }) => (
                  <View>
                    <View className="flex-row items-center mb-1">
                      <Text className="text-xs text-gray-500">Per day ($)</Text>
                      {aiFilledFields.has("pricePerDay") && <AIBadge />}
                    </View>
                    <Input
                      testID="input-listing-price-per-day"
                      placeholder="25"
                      keyboardType="decimal-pad"
                      value={
                        value != null &&
                        value !== ("" as unknown as number)
                          ? String(value)
                          : ""
                      }
                      onChangeText={(text) => {
                        markEdited("pricePerDay");
                        onChange(text);
                        const dollars = parseFloat(text);
                        if (!isNaN(dollars) && onPricePerDayChange) {
                          onPricePerDayChange(dollars);
                        }
                      }}
                      onBlur={onBlur}
                      error={errors.pricePerDay?.message}
                    />
                  </View>
                )}
              />
            </View>
            <View className="flex-1">
              <Controller
                control={control}
                name="pricePerHour"
                render={({ field: { value, onChange, onBlur } }) => (
                  <View>
                    <View className="flex-row items-center mb-1">
                      <Text className="text-xs text-gray-500">
                        Per hour ($ optional)
                      </Text>
                      {aiFilledFields.has("pricePerHour") && <AIBadge />}
                    </View>
                    <Input
                      placeholder="5"
                      keyboardType="decimal-pad"
                      value={
                        value != null &&
                        value !== ("" as unknown as number)
                          ? String(value)
                          : ""
                      }
                      onChangeText={(text) => {
                        markEdited("pricePerHour");
                        onChange(text);
                      }}
                      onBlur={onBlur}
                      error={errors.pricePerHour?.message}
                    />
                  </View>
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
                    testID="input-listing-lat"
                    label="Latitude"
                    placeholder="33.77"
                    keyboardType="decimal-pad"
                    value={
                      value != null && value !== ("" as unknown as number)
                        ? String(value)
                        : ""
                    }
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
                    testID="input-listing-lng"
                    label="Longitude"
                    placeholder="-118.19"
                    keyboardType="decimal-pad"
                    value={
                      value != null && value !== ("" as unknown as number)
                        ? String(value)
                        : ""
                    }
                    onChangeText={onChange}
                    onBlur={onBlur}
                    error={errors.lng?.message}
                  />
                )}
              />
            </View>
          </View>

          <Button
            testID="btn-create-listing"
            title="Create Listing"
            onPress={onFormSubmit}
            loading={isSubmitting}
          />
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
