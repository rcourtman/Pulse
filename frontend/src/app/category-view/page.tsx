"use client";

import React, { useEffect, useState } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Category, Script } from "@/lib/types";

const CategoryView = () => {
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null);

  useEffect(() => {
    const fetchCategoriesAndScripts = async () => {
      try {
        const basePath = process.env.NODE_ENV === "production" ? "/ProxmoxVE" : ""; // Dynamischer Basis-Pfad

        // Kategorien laden
        const categoriesResponse = await fetch(`${basePath}/json/metadata.json`);
        if (!categoriesResponse.ok) {
          throw new Error("Failed to fetch categories");
        }
        const metadata = await categoriesResponse.json();
        console.log("Raw metadata:", metadata); // Debugging

        if (!metadata.categories) {
          throw new Error("Invalid metadata structure: categories missing");
        }

        const categories = metadata.categories.map((category: Category) => ({
          ...category,
          scripts: [],
        }));

        // Skripte laden
        const scriptsResponse = await fetch(`${basePath}/json`);
        if (!scriptsResponse.ok) {
          throw new Error("Failed to fetch scripts");
        }

        const scriptsList = await scriptsResponse.json();
        const scripts: Script[] = await Promise.all(
          scriptsList
            .filter((file: string) => file.endsWith(".json") && file !== "metadata.json")
            .map(async (file: string) => {
              const scriptResponse = await fetch(`${basePath}/json/${file}`);
              if (scriptResponse.ok) {
                return await scriptResponse.json();
              }
              return null;
            })
        ).then((results) => results.filter((script) => script !== null));

        // Kategorien und Skripte verknüpfen
        categories.forEach((category) => {
          category.scripts = scripts.filter((script: Script) =>
            script.categories.includes(category.id)
          );
        });

        console.log("Parsed categories with scripts:", categories); // Debugging
        setCategories(categories);
      } catch (error) {
        console.error("Error fetching categories and scripts:", error);
      }
    };

    fetchCategoriesAndScripts();
  }, []);

  const handleCategoryClick = (category: Category) => {
    setSelectedCategory(category);
  };

  const handleBackClick = () => {
    setSelectedCategory(null);
  };

  return (
    <div className="p-4">
      {categories.length === 0 && (
        <p className="text-center text-gray-500">No categories available. Please check the JSON file.</p>
      )}
      {selectedCategory ? (
        <div>
          <Button variant="default" onClick={handleBackClick} className="mb-4">
            Back to Categories
          </Button>
          <h2 className="text-xl font-semibold mb-4">{selectedCategory.name}</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
            {selectedCategory.scripts
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((script: Script) => (
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
                className="cursor-pointer hover:shadow-lg"
              >
                <CardHeader title={category.name} className="text-lg font-semibold" />
              </Card>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default CategoryView;