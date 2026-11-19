import 'package:flutter/material.dart';
import 'tokens.dart';

enum ReadingThemeMode { bone, sepia, dark }

@immutable
class ShelfdColors extends ThemeExtension<ShelfdColors> {
  final Color background;
  final Color surface;
  final Color containerLow;
  final Color container;
  final Color containerHigh;
  final Color containerHighest;
  final Color ink;
  final Color secondaryInk;
  final Color mutedInk;
  final Color border;
  final Color subtleBorder;
  final Color accentPrimary;
  final Color accentContainer;
  final Color accentSecondary;
  final Color accentSecondaryContainer;
  final Color onAccent;
  final Color errorBg;
  final Color errorBorder;
  final Color errorText;
  final Color warningBg;
  final Color warningBorder;
  final Color warningText;
  final Color cardShadowColor;
  final bool isDark;

  const ShelfdColors({
    required this.background,
    required this.surface,
    required this.containerLow,
    required this.container,
    required this.containerHigh,
    required this.containerHighest,
    required this.ink,
    required this.secondaryInk,
    required this.mutedInk,
    required this.border,
    required this.subtleBorder,
    required this.accentPrimary,
    required this.accentContainer,
    required this.accentSecondary,
    required this.accentSecondaryContainer,
    required this.onAccent,
    required this.errorBg,
    required this.errorBorder,
    required this.errorText,
    required this.warningBg,
    required this.warningBorder,
    required this.warningText,
    required this.cardShadowColor,
    required this.isDark,
  });

  static const ShelfdColors bone = ShelfdColors(
    background: AppTokens.warmParchment,
    surface: AppTokens.warmSurfaceLowest,
    containerLow: AppTokens.warmSurfaceLow,
    container: AppTokens.warmSurfaceContainer,
    containerHigh: AppTokens.warmSurfaceHigh,
    containerHighest: AppTokens.warmSurfaceHighest,
    ink: AppTokens.espressoInk,
    secondaryInk: AppTokens.espressoSecondary,
    mutedInk: AppTokens.espressoMuted,
    border: AppTokens.ghostBorder,
    subtleBorder: AppTokens.subtleGhostBorder,
    accentPrimary: AppTokens.siennaPrimary,
    accentContainer: AppTokens.siennaLightTint,
    accentSecondary: AppTokens.libraryGreen,
    accentSecondaryContainer: AppTokens.libraryGreenContainer,
    onAccent: Colors.white,
    errorBg: AppTokens.errorBgLight,
    errorBorder: AppTokens.errorBorderLight,
    errorText: AppTokens.errorTextLight,
    warningBg: AppTokens.warningBgLight,
    warningBorder: AppTokens.warningBorderLight,
    warningText: AppTokens.warningTextLight,
    cardShadowColor: Color(0x0D1E1B18),
    isDark: false,
  );

  static const ShelfdColors sepia = ShelfdColors(
    background: AppTokens.sepiaBackground,
    surface: AppTokens.sepiaSurface,
    containerLow: AppTokens.sepiaContainerLow,
    container: AppTokens.sepiaContainer,
    containerHigh: AppTokens.sepiaContainerHigh,
    containerHighest: AppTokens.sepiaContainerHighest,
    ink: AppTokens.sepiaInk,
    secondaryInk: AppTokens.sepiaCopy,
    mutedInk: AppTokens.sepiaMuted,
    border: AppTokens.sepiaBorder,
    subtleBorder: AppTokens.sepiaSubtleBorder,
    accentPrimary: AppTokens.siennaPrimary,
    accentContainer: AppTokens.sepiaContainerHigh,
    accentSecondary: AppTokens.libraryGreen,
    accentSecondaryContainer: AppTokens.libraryGreenContainer,
    onAccent: Colors.white,
    errorBg: AppTokens.errorBgLight,
    errorBorder: AppTokens.errorBorderLight,
    errorText: AppTokens.errorTextLight,
    warningBg: AppTokens.warningBgLight,
    warningBorder: AppTokens.warningBorderLight,
    warningText: AppTokens.warningTextLight,
    cardShadowColor: Color(0x145B4636),
    isDark: false,
  );

  static const ShelfdColors dark = ShelfdColors(
    background: AppTokens.walnutBackground,
    surface: AppTokens.walnutSurface,
    containerLow: AppTokens.walnutContainerLow,
    container: AppTokens.walnutContainer,
    containerHigh: AppTokens.walnutElevated,
    containerHighest: AppTokens.walnutHighest,
    ink: AppTokens.walnutInk,
    secondaryInk: AppTokens.walnutSecondary,
    mutedInk: AppTokens.walnutMuted,
    border: AppTokens.walnutBorder,
    subtleBorder: AppTokens.walnutSubtleBorder,
    accentPrimary: AppTokens.siennaDarkAccent,
    accentContainer: Color(0xFF3A261B),
    accentSecondary: AppTokens.libraryGreenDark,
    accentSecondaryContainer: Color(0xFF1E3122),
    onAccent: Color(0xFF161412),
    errorBg: AppTokens.errorBgDark,
    errorBorder: AppTokens.errorBorderDark,
    errorText: AppTokens.errorTextDark,
    warningBg: AppTokens.warningBgDark,
    warningBorder: AppTokens.warningBorderDark,
    warningText: AppTokens.warningTextDark,
    cardShadowColor: Color(0x40000000),
    isDark: true,
  );

  static ShelfdColors of(BuildContext context) {
    final theme = Theme.of(context);
    final ext = theme.extension<ShelfdColors>();
    if (ext != null) return ext;
    if (theme.brightness == Brightness.dark) return ShelfdColors.dark;
    if (theme.scaffoldBackgroundColor == AppTokens.sepiaBackground) {
      return ShelfdColors.sepia;
    }
    return ShelfdColors.bone;
  }

  @override
  ShelfdColors copyWith({
    Color? background,
    Color? surface,
    Color? containerLow,
    Color? container,
    Color? containerHigh,
    Color? containerHighest,
    Color? ink,
    Color? secondaryInk,
    Color? mutedInk,
    Color? border,
    Color? subtleBorder,
    Color? accentPrimary,
    Color? accentContainer,
    Color? accentSecondary,
    Color? accentSecondaryContainer,
    Color? onAccent,
    Color? errorBg,
    Color? errorBorder,
    Color? errorText,
    Color? warningBg,
    Color? warningBorder,
    Color? warningText,
    Color? cardShadowColor,
    bool? isDark,
  }) {
    return ShelfdColors(
      background: background ?? this.background,
      surface: surface ?? this.surface,
      containerLow: containerLow ?? this.containerLow,
      container: container ?? this.container,
      containerHigh: containerHigh ?? this.containerHigh,
      containerHighest: containerHighest ?? this.containerHighest,
      ink: ink ?? this.ink,
      secondaryInk: secondaryInk ?? this.secondaryInk,
      mutedInk: mutedInk ?? this.mutedInk,
      border: border ?? this.border,
      subtleBorder: subtleBorder ?? this.subtleBorder,
      accentPrimary: accentPrimary ?? this.accentPrimary,
      accentContainer: accentContainer ?? this.accentContainer,
      accentSecondary: accentSecondary ?? this.accentSecondary,
      accentSecondaryContainer:
          accentSecondaryContainer ?? this.accentSecondaryContainer,
      onAccent: onAccent ?? this.onAccent,
      errorBg: errorBg ?? this.errorBg,
      errorBorder: errorBorder ?? this.errorBorder,
      errorText: errorText ?? this.errorText,
      warningBg: warningBg ?? this.warningBg,
      warningBorder: warningBorder ?? this.warningBorder,
      warningText: warningText ?? this.warningText,
      cardShadowColor: cardShadowColor ?? this.cardShadowColor,
      isDark: isDark ?? this.isDark,
    );
  }

  @override
  ShelfdColors lerp(ThemeExtension<ShelfdColors>? other, double t) {
    if (other is! ShelfdColors) return this;
    return ShelfdColors(
      background: Color.lerp(background, other.background, t)!,
      surface: Color.lerp(surface, other.surface, t)!,
      containerLow: Color.lerp(containerLow, other.containerLow, t)!,
      container: Color.lerp(container, other.container, t)!,
      containerHigh: Color.lerp(containerHigh, other.containerHigh, t)!,
      containerHighest:
          Color.lerp(containerHighest, other.containerHighest, t)!,
      ink: Color.lerp(ink, other.ink, t)!,
      secondaryInk: Color.lerp(secondaryInk, other.secondaryInk, t)!,
      mutedInk: Color.lerp(mutedInk, other.mutedInk, t)!,
      border: Color.lerp(border, other.border, t)!,
      subtleBorder: Color.lerp(subtleBorder, other.subtleBorder, t)!,
      accentPrimary: Color.lerp(accentPrimary, other.accentPrimary, t)!,
      accentContainer: Color.lerp(accentContainer, other.accentContainer, t)!,
      accentSecondary: Color.lerp(accentSecondary, other.accentSecondary, t)!,
      accentSecondaryContainer: Color.lerp(
        accentSecondaryContainer,
        other.accentSecondaryContainer,
        t,
      )!,
      onAccent: Color.lerp(onAccent, other.onAccent, t)!,
      errorBg: Color.lerp(errorBg, other.errorBg, t)!,
      errorBorder: Color.lerp(errorBorder, other.errorBorder, t)!,
      errorText: Color.lerp(errorText, other.errorText, t)!,
      warningBg: Color.lerp(warningBg, other.warningBg, t)!,
      warningBorder: Color.lerp(warningBorder, other.warningBorder, t)!,
      warningText: Color.lerp(warningText, other.warningText, t)!,
      cardShadowColor: Color.lerp(cardShadowColor, other.cardShadowColor, t)!,
      isDark: t < 0.5 ? isDark : other.isDark,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is ShelfdColors &&
        other.background == background &&
        other.surface == surface &&
        other.containerLow == containerLow &&
        other.container == container &&
        other.containerHigh == containerHigh &&
        other.containerHighest == containerHighest &&
        other.ink == ink &&
        other.secondaryInk == secondaryInk &&
        other.mutedInk == mutedInk &&
        other.border == border &&
        other.subtleBorder == subtleBorder &&
        other.accentPrimary == accentPrimary &&
        other.accentContainer == accentContainer &&
        other.accentSecondary == accentSecondary &&
        other.accentSecondaryContainer == accentSecondaryContainer &&
        other.onAccent == onAccent &&
        other.errorBg == errorBg &&
        other.errorBorder == errorBorder &&
        other.errorText == errorText &&
        other.warningBg == warningBg &&
        other.warningBorder == warningBorder &&
        other.warningText == warningText &&
        other.cardShadowColor == cardShadowColor &&
        other.isDark == isDark;
  }

  @override
  int get hashCode => Object.hashAll([
        background,
        surface,
        containerLow,
        container,
        containerHigh,
        containerHighest,
        ink,
        secondaryInk,
        mutedInk,
        border,
        subtleBorder,
        accentPrimary,
        accentContainer,
        accentSecondary,
        accentSecondaryContainer,
        onAccent,
        errorBg,
        errorBorder,
        errorText,
        warningBg,
        warningBorder,
        warningText,
        cardShadowColor,
        isDark,
      ]);
}

extension ShelfdColorsBuildContext on BuildContext {
  ShelfdColors get shelfdColors => ShelfdColors.of(this);
}

class AppTheme {
  AppTheme._();

  static ShelfdColors colorsOf(BuildContext context) =>
      ShelfdColors.of(context);

  static ThemeData buildTheme(ReadingThemeMode mode) {
    switch (mode) {
      case ReadingThemeMode.bone:
        return _buildThemeData(ShelfdColors.bone, Brightness.light);
      case ReadingThemeMode.sepia:
        return _buildThemeData(ShelfdColors.sepia, Brightness.light);
      case ReadingThemeMode.dark:
        return _buildThemeData(ShelfdColors.dark, Brightness.dark);
    }
  }

  static ThemeData _buildThemeData(ShelfdColors colors, Brightness brightness) {
    final isDark = brightness == Brightness.dark;
    final colorScheme = isDark
        ? ColorScheme.dark(
            primary: colors.accentPrimary,
            onPrimary: colors.onAccent,
            secondary: colors.accentSecondary,
            onSecondary: colors.onAccent,
            surface: colors.surface,
            onSurface: colors.ink,
            onSurfaceVariant: colors.secondaryInk,
            outline: colors.border,
            outlineVariant: colors.subtleBorder,
            error: colors.errorText,
          )
        : ColorScheme.light(
            primary: colors.accentPrimary,
            onPrimary: colors.onAccent,
            secondary: colors.accentSecondary,
            onSecondary: colors.onAccent,
            surface: colors.surface,
            onSurface: colors.ink,
            onSurfaceVariant: colors.secondaryInk,
            outline: colors.border,
            outlineVariant: colors.subtleBorder,
            error: colors.errorText,
          );

    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      scaffoldBackgroundColor: colors.background,
      cardColor: colors.surface,
      dividerColor: colors.border,
      extensions: <ThemeExtension<dynamic>>[colors],
      colorScheme: colorScheme,
      textTheme: TextTheme(
        bodyMedium: TextStyle(color: colors.ink),
        bodyLarge: TextStyle(color: colors.ink),
        bodySmall: TextStyle(color: colors.secondaryInk),
        titleMedium: TextStyle(color: colors.ink),
        titleLarge: TextStyle(color: colors.ink),
      ),
      appBarTheme: AppBarTheme(
        backgroundColor: colors.background,
        foregroundColor: colors.ink,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        scrolledUnderElevation: 0,
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: colors.surface,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusLg),
          side: BorderSide(color: colors.border),
        ),
      ),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: colors.surface,
        surfaceTintColor: Colors.transparent,
        modalBackgroundColor: colors.surface,
      ),
      cardTheme: CardThemeData(
        color: colors.surface,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        margin: EdgeInsets.zero,
      ),
      dividerTheme: DividerThemeData(
        color: colors.border,
        thickness: 1,
        space: 1,
      ),
      popupMenuTheme: PopupMenuThemeData(
        color: colors.surface,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          side: BorderSide(color: colors.border),
        ),
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: colors.ink,
        contentTextStyle: TextStyle(color: colors.surface),
        behavior: SnackBarBehavior.floating,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        ),
      ),
      switchTheme: SwitchThemeData(
        trackColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return colors.accentPrimary;
          }
          return colors.containerHigh;
        }),
        thumbColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return colors.onAccent;
          }
          return colors.secondaryInk;
        }),
        trackOutlineColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return Colors.transparent;
          }
          return colors.border;
        }),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: colors.ink,
          side: BorderSide(color: colors.border),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          ),
          padding: const EdgeInsets.symmetric(
            horizontal: AppTokens.space16,
            vertical: AppTokens.space12,
          ),
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: colors.accentPrimary,
          foregroundColor: colors.onAccent,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          ),
          padding: const EdgeInsets.symmetric(
            horizontal: AppTokens.space16,
            vertical: AppTokens.space12,
          ),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: colors.containerLow,
        hintStyle: TextStyle(color: colors.mutedInk),
        labelStyle: TextStyle(color: colors.secondaryInk),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: AppTokens.space12,
          vertical: AppTokens.space12,
        ),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          borderSide: BorderSide(color: colors.border),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          borderSide: BorderSide(color: colors.border),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          borderSide: BorderSide(color: colors.errorBorder),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          borderSide: BorderSide(color: colors.errorText, width: 1.5),
        ),
      ),
      chipTheme: ChipThemeData(
        backgroundColor: colors.containerLow,
        selectedColor: colors.accentContainer,
        side: BorderSide(color: colors.border),
        labelStyle: TextStyle(color: colors.ink),
        secondaryLabelStyle: TextStyle(color: colors.accentPrimary),
      ),
    );
  }
}
