"use client";

import React, { useEffect, useState } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { fetchCategories } from "@/lib/data";
import { Category } from "@/lib/types";

const CategoryView = () => {
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null);

  useEffect(() => {
    fetchCategories()
      .then(setCategories)
      .catch((error) => console.error("Error fetching categories:", error));
  }, []);

  const handleCategoryClick = (category: Category) => {
    setSelectedCategory(category);
  };

  const handleBackClick = () => {
    setSelectedCategory(null);
  };

  return (
    <div className="p-4">
      {selectedCategory ? (
        <div>
          <Button variant="primary" onClick={handleBackClick} className="mb-4">
            Back to Categories
          </Button>
          <h2 className="text-xl font-semibold mb-4">{selectedCategory.name}</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
            {selectedCategory.scripts
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((script) => (
                <Card key={script.name}>
                  <CardContent>
                    <h3 className="text-lg font-medium">{script.name}</h3>
                    <p className="text-sm text-gray-600">{script.date || "N/A"}</p>
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