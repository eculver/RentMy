import { Pressable, Text, ActivityIndicator } from "react-native";

interface ButtonProps {
  title: string;
  onPress: () => void;
  variant?: "primary" | "secondary" | "ghost";
  disabled?: boolean;
  loading?: boolean;
  testID?: string;
}

const variantStyles = {
  primary: {
    container: "bg-primary-600 py-4 rounded-xl items-center",
    containerDisabled: "bg-primary-300 py-4 rounded-xl items-center",
    text: "text-white font-semibold text-lg",
  },
  secondary: {
    container: "border border-primary-600 py-4 rounded-xl items-center",
    containerDisabled: "border border-gray-300 py-4 rounded-xl items-center",
    text: "text-primary-600 font-semibold text-lg",
  },
  ghost: {
    container: "py-4 rounded-xl items-center",
    containerDisabled: "py-4 rounded-xl items-center",
    text: "text-primary-600 font-medium text-base",
  },
};

export default function Button({ title, onPress, variant = "primary", disabled = false, loading = false, testID }: ButtonProps) {
  const styles = variantStyles[variant];
  const isDisabled = disabled || loading;

  return (
    <Pressable
      className={isDisabled ? styles.containerDisabled : styles.container}
      onPress={onPress}
      disabled={isDisabled}
      testID={testID}
    >
      {loading ? (
        <ActivityIndicator color={variant === "primary" ? "#fff" : "#0284c7"} />
      ) : (
        <Text className={styles.text}>{title}</Text>
      )}
    </Pressable>
  );
}
