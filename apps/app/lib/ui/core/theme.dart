import 'package:flutter/material.dart';
import 'tokens.dart';

enum ReadingThemeMode { bone, sepia, dark }

class AppTheme {
  AppTheme._();

  static ThemeData buildTheme(ReadingThemeMode mode) {
    switch (mode) {
      case ReadingThemeMode.bone:
        return ThemeData(
          useMaterial3: true,
          brightness: Brightness.light,
          scaffoldBackgroundColor: AppTokens.boneBackground,
          cardColor: AppTokens.boneSurface,
          dividerColor: AppTokens.crispBorder,
          colorScheme: const ColorScheme.light(
            primary: AppTokens.charcoalInk,
            onPrimary: Colors.white,
            surface: AppTokens.boneSurface,
            onSurface: AppTokens.charcoalInk,
            onSurfaceVariant: AppTokens.mutedCopy,
            outline: AppTokens.crispBorder,
          ),
          textTheme: const TextTheme(
            bodyMedium: TextStyle(color: AppTokens.charcoalInk),
            bodyLarge: TextStyle(color: AppTokens.charcoalInk),
            bodySmall: TextStyle(color: AppTokens.mutedCopy),
            titleMedium: TextStyle(color: AppTokens.charcoalInk),
            titleLarge: TextStyle(color: AppTokens.charcoalInk),
          ),
          appBarTheme: const AppBarTheme(
            backgroundColor: AppTokens.boneBackground,
            foregroundColor: AppTokens.charcoalInk,
            elevation: 0,
            scrolledUnderElevation: 0,
          ),
        );

      case ReadingThemeMode.sepia:
        return ThemeData(
          useMaterial3: true,
          brightness: Brightness.light,
          scaffoldBackgroundColor: AppTokens.sepiaBackground,
          cardColor: AppTokens.sepiaSurface,
          dividerColor: AppTokens.sepiaBorder,
          colorScheme: const ColorScheme.light(
            primary: AppTokens.sepiaInk,
            onPrimary: Colors.white,
            surface: AppTokens.sepiaSurface,
            onSurface: AppTokens.sepiaInk,
            onSurfaceVariant: AppTokens.sepiaCopy,
            outline: AppTokens.sepiaBorder,
          ),
          textTheme: const TextTheme(
            bodyMedium: TextStyle(color: AppTokens.sepiaInk),
            bodyLarge: TextStyle(color: AppTokens.sepiaInk),
            bodySmall: TextStyle(color: AppTokens.sepiaCopy),
            titleMedium: TextStyle(color: AppTokens.sepiaInk),
            titleLarge: TextStyle(color: AppTokens.sepiaInk),
          ),
          appBarTheme: const AppBarTheme(
            backgroundColor: AppTokens.sepiaBackground,
            foregroundColor: AppTokens.sepiaInk,
            elevation: 0,
            scrolledUnderElevation: 0,
          ),
        );

      case ReadingThemeMode.dark:
        return ThemeData(
          useMaterial3: true,
          brightness: Brightness.dark,
          scaffoldBackgroundColor: AppTokens.darkOledBackground,
          cardColor: AppTokens.darkOledSurface,
          dividerColor: AppTokens.darkOledBorder,
          colorScheme: const ColorScheme.dark(
            primary: AppTokens.darkOledInk,
            onPrimary: Colors.black,
            surface: AppTokens.darkOledSurface,
            onSurface: AppTokens.darkOledInk,
            onSurfaceVariant: AppTokens.darkOledCopy,
            outline: AppTokens.darkOledBorder,
          ),
          textTheme: const TextTheme(
            bodyMedium: TextStyle(color: AppTokens.darkOledInk),
            bodyLarge: TextStyle(color: AppTokens.darkOledInk),
            bodySmall: TextStyle(color: AppTokens.darkOledCopy),
            titleMedium: TextStyle(color: AppTokens.darkOledInk),
            titleLarge: TextStyle(color: AppTokens.darkOledInk),
          ),
          appBarTheme: const AppBarTheme(
            backgroundColor: AppTokens.darkOledBackground,
            foregroundColor: AppTokens.darkOledInk,
            elevation: 0,
            scrolledUnderElevation: 0,
          ),
        );
    }
  }
}
