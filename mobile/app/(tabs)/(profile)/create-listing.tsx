import { useState } from "react";
import { View, Text } from "react-native";
import { router } from "expo-router";
import { HTTPError } from "ky";
import { api } from "../../../lib/api";
import AngleEnforcedCamera, {
  type CapturedPhoto,
} from "../../../components/camera/AngleEnforcedCamera";
import ListingForm, {
  type ListingFormData,
  type AISuggestions,
} from "../../../components/listing/ListingForm";
import AIAutofillOverlay from "../../../components/listing/AIAutofillOverlay";
import ValueOverridePrompt from "../../../components/listing/ValueOverridePrompt";
import { useAppraisal } from "../../../lib/hooks/useAppraisal";

type Step = "camera" | "form";

interface MediaResponse {
  id: string;
  url: string;
  thumbnailUrl: string;
}

interface ListingResponse {
  id: string;
  title: string;
}

// 2x threshold: if host price/day exceeds 2x AI estimated daily value, prompt for override
const OVERRIDE_THRESHOLD_MULTIPLIER = 2;
// Rough conversion: estimated item value / 30 as a proxy for daily rental price
const DAILY_PRICE_DIVISOR = 30;

export default function CreateListingScreen() {
  const [step, setStep] = useState<Step>("camera");
  const [captures, setCaptures] = useState<CapturedPhoto[]>([]);
  const [apiError, setApiError] = useState<string | null>(null);
  const [listingId, setListingId] = useState<string | null>(null);
  const [aiSuggestions, setAiSuggestions] = useState<AISuggestions | null>(
    null
  );
  const [overrideVisible, setOverrideVisible] = useState(false);
  const [pendingPriceDollars, setPendingPriceDollars] = useState<number | null>(
    null
  );

  const { data: appraisal, isLoading: appraisalLoading } =
    useAppraisal(listingId);

  // When appraisal completes, apply suggestions to the form
  const appraisalComplete =
    !!appraisal && appraisal.status === "COMPLETE" && !appraisalLoading;
  const appraisalFailed = !!appraisal && appraisal.status === "FAILED";
  const showOverlay =
    !!listingId && (appraisalLoading || appraisal?.status === "PENDING");

  const resolvedSuggestions: AISuggestions | null = appraisalComplete
    ? {
        title: appraisal.itemName,
        description: appraisal.description,
        pricePerDay: appraisal.suggestedPricePerDayCents
          ? appraisal.suggestedPricePerDayCents / 100
          : undefined,
        pricePerHour: appraisal.suggestedPricePerHourCents
          ? appraisal.suggestedPricePerHourCents / 100
          : undefined,
        tags: Array.isArray(appraisal.tags)
          ? (appraisal.tags as string[])
          : undefined,
        estimatedValueCents: appraisal.estimatedValueCents,
      }
    : aiSuggestions;

  const handleCapture = (photo: CapturedPhoto) => {
    setCaptures((prev) => [...prev, photo]);
  };

  const handlePhotoDone = () => {
    setStep("form");
  };

  const handleFormSubmit = async (formData: ListingFormData) => {
    setApiError(null);
    try {
      // 1. Upload each captured photo
      const mediaIds: string[] = [];
      for (const capture of captures) {
        const fd = new FormData();
        const uri = capture.path.startsWith("file://")
          ? capture.path
          : `file://${capture.path}`;
        fd.append("image", {
          uri,
          type: "image/jpeg",
          name: "photo.jpg",
        } as unknown as Blob);
        fd.append("orientation", JSON.stringify(capture.orientation));
        const media = await api
          .post("api/v1/media/upload", { body: fd })
          .json<MediaResponse>();
        mediaIds.push(media.id);
      }

      // 2. Create the listing (triggers AI appraisal via River job on the backend)
      const listing = await api
        .post("api/v1/listings", {
          json: {
            title: formData.title,
            description: formData.description,
            pricePerDay: formData.pricePerDay,
            ...(formData.pricePerHour != null && {
              pricePerHour: formData.pricePerHour,
            }),
            maxDuration: formData.maxDuration,
            location: { lat: formData.lat, lng: formData.lng },
          },
        })
        .json<ListingResponse>();

      // 3. Attach uploaded media to the listing
      if (mediaIds.length > 0) {
        await api
          .post(`api/v1/listings/${listing.id}/media`, {
            json: { mediaIds },
          })
          .json();
      }

      // 4. Track listing ID to start polling for appraisal
      setListingId(listing.id);
    } catch (err) {
      if (err instanceof HTTPError) {
        const status = err.response.status;
        if (status === 400) {
          setApiError("Invalid listing data. Please check your inputs.");
        } else if (status === 413) {
          setApiError("One or more photos are too large. Please try again.");
        } else {
          setApiError("Failed to create listing. Please try again.");
        }
      } else {
        setApiError("Failed to create listing. Please try again.");
      }
    }
  };

  // If appraisal is complete and listing ID exists, we can navigate away
  const handleContinueAfterAppraisal = () => {
    router.back();
  };

  const handlePricePerDayChange = (dollars: number) => {
    setPendingPriceDollars(dollars);
    if (!resolvedSuggestions?.estimatedValueCents) return;

    const aiDailyValueDollars =
      resolvedSuggestions.estimatedValueCents /
      100 /
      DAILY_PRICE_DIVISOR;
    if (dollars > aiDailyValueDollars * OVERRIDE_THRESHOLD_MULTIPLIER) {
      setOverrideVisible(true);
    }
  };

  if (step === "camera") {
    return (
      <View className="flex-1 bg-black">
        <AngleEnforcedCamera
          captures={captures}
          onCapture={handleCapture}
          onDone={handlePhotoDone}
          maxPhotos={6}
        />
      </View>
    );
  }

  // After listing is created, poll for appraisal result
  if (listingId) {
    if (showOverlay) {
      return (
        <View className="flex-1 bg-white">
          <AIAutofillOverlay isLoading={true} />
        </View>
      );
    }

    if (appraisalFailed) {
      return (
        <View className="flex-1 bg-white">
          <AIAutofillOverlay
            isLoading={false}
            error={appraisal?.failureReason ?? "Unknown error"}
          />
          <View className="px-6 pb-10 mt-auto">
            <Text
              className="text-center text-sky-600 text-sm font-semibold py-3"
              onPress={handleContinueAfterAppraisal}
            >
              Continue without AI suggestions
            </Text>
          </View>
        </View>
      );
    }

    // Appraisal complete — navigate back (listing is ready)
    if (appraisalComplete && !overrideVisible) {
      router.back();
      return null;
    }
  }

  return (
    <View className="flex-1 bg-white">
      {apiError && (
        <View className="px-6 pt-4">
          <Text className="text-red-500 text-sm text-center">{apiError}</Text>
        </View>
      )}
      <ListingForm
        onSubmit={handleFormSubmit}
        aiSuggestions={resolvedSuggestions}
        onPricePerDayChange={handlePricePerDayChange}
      />
      {listingId && overrideVisible && resolvedSuggestions?.estimatedValueCents && pendingPriceDollars != null && (
        <ValueOverridePrompt
          visible={overrideVisible}
          listingId={listingId}
          aiEstimateCents={resolvedSuggestions.estimatedValueCents}
          hostValueCents={Math.round(pendingPriceDollars * 100)}
          onClose={() => setOverrideVisible(false)}
          onResult={() => setOverrideVisible(false)}
        />
      )}
    </View>
  );
}
