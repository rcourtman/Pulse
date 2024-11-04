import { pb } from "@/lib/pocketbase";
import { Category } from "@/lib/types";
import { NextResponse } from "next/server";

export const dynamic = "force-static";

export async function GET() {
  try {
    const response = await pb.collection("categories").getFullList<Category>({
      expand: "items.alerts,items.alpine_script,items.default_login",
      sort: "order",
    });

    return NextResponse.json(response);
  } catch (error) {
    console.error("Error fetching categories:", error);
    return NextResponse.json(
      { error: "Failed to fetch categories" },
      { status: 500 },
    );
  }
}

