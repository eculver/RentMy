import { useState } from "react";
import { View, Text, ScrollView } from "react-native";
import { router } from "expo-router";
import { HTTPError } from "ky";
import { api } from "../../../lib/api";
import AngleEnforcedCamera, {
  type CapturedPhoto,
} from "../../../components/camera/AngleEnforcedCamera";
import ListingForm, {
  type ListingFormData,
} from "../../../components/listing/ListingForm";

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

export default function CreateListingScreen() {
  const [step, setStep] = useState<Step>("camera");
  const [captures, setCaptures] = useState<CapturedPhoto[]>([]);
  const [apiError, setApiError] = useState<string | null>(null);

  const handleCapture = (photo: CapturedPhoto) => {
    setCaptures((prev) => [...prev, photo]);
  };

  const handleFormSubmit = async (formData: ListingFormData) => {
    setApiError(null);
    try {
      // 1. Upload each captured photo with its orientation metadata
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

      // 2. Create the listing
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

      router.back();
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

  if (step === "camera") {
    return (
      <View className="flex-1 bg-black">
        <AngleEnforcedCamera
          captures={captures}
          onCapture={handleCapture}
          onDone={() => setStep("form")}
          maxPhotos={6}
        />
      </View>
    );
  }

  return (
    <View className="flex-1 bg-white">
      {apiError && (
        <View className="px-6 pt-4">
          <Text className="text-red-500 text-sm text-center">{apiError}</Text>
        </View>
      )}
      <ListingForm onSubmit={handleFormSubmit} />
    </View>
  );
}
