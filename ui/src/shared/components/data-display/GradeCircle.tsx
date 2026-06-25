import type { ProductGrade } from '@/shared/utils/productGrade';
import { GRADE_COLORS } from '@/shared/utils/productGrade';

interface GradeCircleProps {
  grade: ProductGrade;
  size?: number;
}

export function GradeCircle({ grade, size = 48 }: GradeCircleProps) {
  const color = GRADE_COLORS[grade];

  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        border: `2px solid ${color}`,
        background: `${color}15`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color,
        fontSize: size * 0.38,
        fontWeight: 700,
        fontFamily: "'Inter', sans-serif",
      }}
    >
      {grade}
    </div>
  );
}
