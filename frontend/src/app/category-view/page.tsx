"use client";

import React, { useEffect, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Category } from "@/lib/types";

const defaultLogo = "/default-logo.png"; // Fallback logo path

const CategoryView = () => {
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null);

  useEffect(() => {
    const fetchCategories = async () => {
      try {
        const basePath = process.env.NODE_ENV === "production" ? "/ProxmoxVE" : "";
        const response = await fetch(`${basePath}/api/categories`); // Prüfe den Endpunkt
        if (!response.ok) {
          throw new Error("Failed to fetch categories");
        }
        const data = await response.json();
        console.log("Fetched categories:", data); // Debugging
        setCategories(data);
      } catch (error) {
        console.error("Error fetching categories:", error);
      }
    };

    fetchCategories();
  }, []);

  const handleCategoryClick = (category: Category) => {
    setSelectedCategory(category);
  };

  const handleBackClick = () => {
    setSelectedCategory(null);
  };

  return (
    <div className="p-4">
      {categories.length === 0 ? (
        <p className="text-center text-gray-500">
          No categories available. Please check the API endpoint.
        </p>
      ) : selectedCategory ? (
        <div>
          <Button variant="default" onClick={handleBackClick} className="mb-4">
            Back to Categories
          </Button>
          <h2 className="text-xl font-semibold mb-4">{selectedCategory.name}</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
            {(selectedCategory.scripts || [])
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((script) => (
                <Card key={script.name}>
                  <CardContent>
                    <h3 className="text-lg font-medium">{script.name}</h3>
                    <p className="text-sm text-gray-600">
                      {script.date_created || "No date available"}
                    </p>
                  </CardContent>
                </Card>
              ))}
          </div>
        </div>
      ) : (
        <div>
          <h1 className="text-2xl font-bold mb-6">Categories</h1>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
            {categories.map((category) => (
              <Card
                key={category.name}
                onClick={() => handleCategoryClick(category)}
                className="cursor-pointer hover:shadow-lg flex flex-col items-center justify-center"
              >
                <CardContent className="flex flex-col items-center">
                  <div className="flex flex-wrap justify-center gap-1 mb-2">
                    {(category.scripts || []).slice(0, 4).map((script, index) => (
                      <img
                        key={index}
                        src={script.logo || defaultLogo}
                        alt={script.name || "Script logo"}
                        className="h-6 w-6 object-contain"
                      />
                    ))}
                  </div>
                  <h3 className="text-lg font-bold mb-1">{category.name}</h3>
                  <p className="text-sm text-gray-500 text-center">
                    {(category as any).description || "No description available."}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default CategoryView;
