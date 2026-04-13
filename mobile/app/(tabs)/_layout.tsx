import { Tabs, Redirect } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useUnreadCount } from "../../lib/hooks/useConversations";
import { useAuthStore } from "../../lib/auth";

type IconName = React.ComponentProps<typeof Ionicons>["name"];

function TabIcon({ name, color, size }: { name: IconName; color: string; size: number }) {
  return <Ionicons name={name} size={size} color={color} />;
}

export default function TabLayout() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const { data: unreadData } = useUnreadCount();
  const unreadCount = unreadData?.count ?? 0;

  if (!isAuthenticated) {
    return <Redirect href="/(auth)/login" />;
  }

  return (
    <Tabs
      screenOptions={{
        tabBarActiveTintColor: "#0284c7",
        tabBarInactiveTintColor: "#9ca3af",
        headerShown: true,
      }}
    >
      <Tabs.Screen
        name="(feed)"
        options={{
          title: "Feed",

          tabBarIcon: ({ color, size }) => <TabIcon name="home-outline" color={color} size={size} />,
        }}
      />
      <Tabs.Screen
        name="(search)"
        options={{
          title: "Search",

          tabBarIcon: ({ color, size }) => <TabIcon name="search-outline" color={color} size={size} />,
        }}
      />
      <Tabs.Screen
        name="(map)"
        options={{
          title: "Map",

          tabBarIcon: ({ color, size }) => <TabIcon name="map-outline" color={color} size={size} />,
        }}
      />
      <Tabs.Screen
        name="(rentals)"
        options={{
          title: "Rentals",

          tabBarIcon: ({ color, size }) => <TabIcon name="receipt-outline" color={color} size={size} />,
        }}
      />
      <Tabs.Screen
        name="(messages)"
        options={{
          title: "Messages",

          tabBarIcon: ({ color, size }) => <TabIcon name="chatbubble-outline" color={color} size={size} />,
          tabBarBadge: unreadCount > 0 ? (unreadCount > 99 ? "99+" : unreadCount) : undefined,
        }}
      />
      <Tabs.Screen
        name="(profile)"
        options={{
          title: "Profile",

          tabBarIcon: ({ color, size }) => <TabIcon name="person-outline" color={color} size={size} />,
        }}
      />
    </Tabs>
  );
}
