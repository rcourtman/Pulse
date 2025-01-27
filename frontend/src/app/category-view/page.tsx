"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Category } from "@/lib/types";

const defaultLogo = "/default-logo.png"; // Fallback logo path

const MAX_DESCRIPTION_LENGTH = 100; // Set max length for description

const CategoryView = () => {
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null);
  const router = useRouter();

  useEffect(() => {
    const fetchCategories = async () => {
      try {
        const basePath = process.env.NODE_ENV === "production" ? "/ProxmoxVE" : "";
        const response = await fetch(`${basePath}/api/categories`);
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

  const handleScriptClick = (scriptSlug: string) => {
    router.push(`/scripts?id=${scriptSlug}`);
  };

  const truncateDescription = (text: string) => {
    return text.length > MAX_DESCRIPTION_LENGTH
      ? `${text.slice(0, MAX_DESCRIPTION_LENGTH)}...`
      : text;
  };

  const getRandomScripts = (scripts: any[]) => {
    if (!scripts || scripts.length <= 5) return scripts;
    return scripts.sort(() => 0.5 - Math.random()).slice(0, 5);
  };

  return (
    <div className="p-6 mt-20">
      {categories.length === 0 && (
        <p className="text-center text-gray-500">No categories available. Please check the API endpoint.</p>
      )}
      {selectedCategory ? (
        <div>
          <Button variant="default" onClick={handleBackClick} className="mb-6">
            Back to Categories
          </Button>
          <h2 className="text-3xl font-semibold mb-6">{selectedCategory.name}</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
            {selectedCategory.scripts
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((script) => (
                <Card
                  key={script.name}
                  className="p-4 cursor-pointer"
                  onClick={() => handleScriptClick(script.slug)}
                >
                  <CardContent className="flex flex-col gap-4">
                    <div>
                      <h3 className="text-lg font-bold">{script.name}</h3>
                      <p className="text-sm text-gray-500">
                        <b>Created at:</b> {script.date_created || "No date available"}
                      </p>
                    </div>
                    <div className="flex flex-wrap justify-center gap-2">
                      <img
                        src={script.logo || defaultLogo}
                        alt={script.name}
                        className="h-12 w-12 object-contain"
                      />
                    </div>
                    <div>
                      <p className="text-sm text-gray-700">
                        {truncateDescription(script.description || "No description available.")}
                      </p>
                      <div className="text-sm text-gray-600 mt-2">
                        <b>CPU:</b> {script.install_methods[0]?.resources.cpu || "N/A"}vCPU |{" "}
                        <b>RAM:</b> {script.install_methods[0]?.resources.ram || "N/A"}MB |{" "}
                        <b>HDD:</b> {script.install_methods[0]?.resources.hdd || "N/A"}GB
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
          </div>
        </div>
      ) : (
        <div>
          <div className="flex justify-between items-center mb-8">
            <h1 className="text-4xl font-bold">Categories</h1>
            <p className="text-sm text-gray-500">
              {categories.reduce((acc, cat) => acc + (cat.scripts?.length || 0), 0)} Total scripts
            </p>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-8">
            {categories.map((category) => (
              <Card
                key={category.name}
                onClick={() => handleCategoryClick(category)}
                className="cursor-pointer hover:shadow-lg flex flex-col items-center justify-center py-6"
              >
                <CardContent className="flex flex-col items-center">
                  <h3 className="text-xl font-bold mb-4">{category.name}</h3>
                  <div className="flex flex-wrap justify-center gap-3 mb-4">
                    {category.scripts &&
                      getRandomScripts(category.scripts).map((script, index) => (
                        <img
                          key={index}
                          src={script.logo || defaultLogo}
                          alt={script.name || "Script logo"}
                          title={script.name} // Tooltip on hover
                          onClick={(e) => {
                            e.stopPropagation(); // Prevent card click
                            handleScriptClick(script.slug);
                          }}
                          className="h-8 w-8 object-contain cursor-pointer hover:scale-110 transition-transform"
                        />
                      ))}
                  </div>
                  <p className="text-sm text-gray-400 text-center">
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
