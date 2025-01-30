"use client";

import React from "react";
import { Pie } from "react-chartjs-2";
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from "chart.js";

ChartJS.register(ArcElement, Tooltip, Legend);

interface ApplicationChartProps {
  data: { nsapp: string }[];
}

const ApplicationChart: React.FC<ApplicationChartProps> = ({ data }) => {
  const chartData = () => {
    const appCounts: Record<string, number> = {};

    data.forEach((item) => {
      appCounts[item.nsapp] = (appCounts[item.nsapp] || 0) + 1;
    });

    return {
      labels: Object.keys(appCounts),
      datasets: [
        {
          label: "Applications",
          data: Object.values(appCounts),
          backgroundColor: [
            "#ff6384",
            "#36a2eb",
            "#ffce56",
            "#4bc0c0",
            "#9966ff",
            "#ff9f40",
          ],
        },
      ],
    };
  };

  return (
    <div className="w-1/2 mx-auto my-6">
      <Pie data={chartData()} />
    </div>
  );
};

export default ApplicationChart;
