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
            outline: AppTokens.crispBorder,
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
            outline: AppTokens.sepiaBorder,
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
            outline: AppTokens.darkOledBorder,
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
