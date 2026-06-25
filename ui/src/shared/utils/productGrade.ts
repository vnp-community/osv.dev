export type ProductGrade = 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'D' | 'F';

export const GRADE_COLORS: Record<ProductGrade, string> = {
  'A': '#10B981',
  'A-': '#34D399',
  'B+': '#60A5FA',
  'B': '#3B82F6',
  'B-': '#818CF8',
  'C+': '#EAB308',
  'C': '#F97316',
  'D': '#EF4444',
  'F': '#7F1D1D',
};

export function calculateProductGrade(findings: {
  critical: number;
  high: number;
  medium: number;
  total: number;
}): { grade: ProductGrade; score: number } {
  const { critical, high, medium } = findings;

  const score = Math.max(
    0,
    100 - (critical * 15) - (high * 5) - (medium * 1)
  );

  let grade: ProductGrade;
  if (score >= 95) grade = 'A';
  else if (score >= 90) grade = 'A-';
  else if (score >= 85) grade = 'B+';
  else if (score >= 80) grade = 'B';
  else if (score >= 75) grade = 'B-';
  else if (score >= 65) grade = 'C+';
  else if (score >= 55) grade = 'C';
  else if (score >= 40) grade = 'D';
  else grade = 'F';

  return { grade, score };
}

export function getGradeColor(grade: ProductGrade): string {
  return GRADE_COLORS[grade] ?? '#6B7280';
}
