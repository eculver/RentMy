import { useState } from "react";
import {
  View,
  Text,
  FlatList,
  Pressable,
  ActivityIndicator,
  Share,
  Platform,
  Clipboard,
} from "react-native";
import { useReferralCode, useMyReferrals, Referral } from "../../../lib/hooks/useReferrals";
import ReferralCard from "../../../components/referral/ReferralCard";

export default function ReferralsScreen() {
  const { data: codeData, isLoading: codeLoading } = useReferralCode();
  const { data: referralsData, isLoading: referralsLoading } = useMyReferrals();
  const [copied, setCopied] = useState(false);

  const referrals: Referral[] = referralsData?.referrals ?? [];

  const handleCopy = () => {
    if (!codeData?.code) return;
    Clipboard.setString(codeData.code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleShare = async () => {
    if (!codeData?.code) return;
    const deepLink = `rentmy://join?ref=${codeData.code}`;
    await Share.share({
      message: `Join me on RentMy and we both get $20 when you complete your first rental! Use my code: ${codeData.code}\n${deepLink}`,
      url: Platform.OS === "ios" ? deepLink : undefined,
    });
  };

  return (
    <View className="flex-1 bg-white">
      {/* Header */}
      <View className="px-6 pt-14 pb-6 border-b border-gray-100">
        <Text className="text-2xl font-bold text-gray-900">Invite Friends</Text>
        <Text className="text-sm text-gray-500 mt-1">
          You and your friend each earn $20 when they complete their first rental.
        </Text>
      </View>

      {/* Referral code card */}
      <View className="mx-6 mt-5 p-5 bg-sky-50 rounded-2xl border border-sky-100">
        {codeLoading ? (
          <ActivityIndicator size="small" color="#0284c7" />
        ) : (
          <>
            <Text className="text-xs text-sky-500 font-semibold uppercase tracking-widest mb-2">
              Your Referral Code
            </Text>
            <Text className="text-3xl font-bold text-sky-700 tracking-widest mb-4">
              {codeData?.code ?? "—"}
            </Text>

            <View className="flex-row gap-x-3">
              <Pressable
                className="flex-1 bg-sky-600 py-3 rounded-xl items-center"
                onPress={handleCopy}
              >
                <Text className="text-white font-semibold text-sm">
                  {copied ? "Copied!" : "Copy Code"}
                </Text>
              </Pressable>

              <Pressable
                className="flex-1 border border-sky-600 py-3 rounded-xl items-center"
                onPress={handleShare}
              >
                <Text className="text-sky-600 font-semibold text-sm">Share</Text>
              </Pressable>
            </View>
          </>
        )}
      </View>

      {/* Referrals list */}
      <View className="flex-1 px-6 mt-5">
        <Text className="text-base font-semibold text-gray-900 mb-3">
          Your Referrals
          {referrals.length > 0 && (
            <Text className="text-gray-400"> ({referrals.length})</Text>
          )}
        </Text>

        {referralsLoading && (
          <ActivityIndicator size="small" color="#0284c7" className="mt-4" />
        )}

        {!referralsLoading && referrals.length === 0 && (
          <View className="items-center mt-10">
            <Text className="text-4xl mb-3">👋</Text>
            <Text className="text-base font-semibold text-gray-700 mb-1">
              No referrals yet
            </Text>
            <Text className="text-sm text-gray-400 text-center px-4">
              Share your code and start earning $20 for every friend who completes a rental.
            </Text>
          </View>
        )}

        {!referralsLoading && referrals.length > 0 && (
          <FlatList
            data={referrals}
            keyExtractor={(item) => item.id}
            renderItem={({ item }) => <ReferralCard referral={item} />}
            showsVerticalScrollIndicator={false}
            contentContainerStyle={{ paddingBottom: 100 }}
          />
        )}
      </View>
    </View>
  );
}
