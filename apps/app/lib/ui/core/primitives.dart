import 'package:flutter/material.dart';
import 'theme.dart';
import 'tokens.dart';
import 'typography.dart';

/// Semantic tone for [ShelfdInlineBanner].
enum ShelfdBannerTone { info, success, warning, error }

/// Consistent desktop page header with serif title, subtitle, and optional trailing actions.
class ShelfdPageHeader extends StatelessWidget {
  final String title;
  final String? subtitle;
  final Widget? trailing;

  const ShelfdPageHeader({
    super.key,
    required this.title,
    this.subtitle,
    this.trailing,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return Wrap(
      alignment: WrapAlignment.spaceBetween,
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: AppTokens.space16,
      runSpacing: AppTokens.space12,
      children: [
        Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              title,
              style: AppTypography.displaySerif(
                fontSize: 28,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            if (subtitle != null) ...[
              const SizedBox(height: AppTokens.space4),
              Text(
                subtitle!,
                style: AppTypography.bodySans(
                  fontSize: 14,
                  color: colors.secondaryInk,
                ),
              ),
            ],
          ],
        ),
        if (trailing != null) trailing!,
      ],
    );
  }
}

/// Editorial surface container with optional structured header (icon, title, subtitle, trailing).
class ShelfdSectionCard extends StatelessWidget {
  final IconData? icon;
  final String? title;
  final String? subtitle;
  final Widget? trailing;
  final Widget child;
  final EdgeInsetsGeometry padding;

  const ShelfdSectionCard({
    super.key,
    this.icon,
    this.title,
    this.subtitle,
    this.trailing,
    required this.child,
    this.padding = const EdgeInsets.all(AppTokens.space20),
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final hasHeader = title != null || trailing != null;

    return Container(
      width: double.infinity,
      padding: padding,
      decoration: BoxDecoration(
        color: colors.surface,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(color: colors.subtleBorder),
        boxShadow: [
          BoxShadow(
            color: colors.cardShadowColor,
            blurRadius: 12,
            offset: const Offset(0, 3),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (hasHeader) ...[
            LayoutBuilder(
              builder: (context, constraints) {
                final titleRow = Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (icon != null) ...[
                      Icon(icon, size: 20, color: colors.accentPrimary),
                      const SizedBox(width: AppTokens.space8),
                    ],
                    if (title != null)
                      Flexible(
                        child: Text(
                          title!,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.titleSerif(
                            fontSize: 18,
                            color: colors.ink,
                          ),
                        ),
                      ),
                  ],
                );

                if (trailing == null) {
                  return Row(children: [Expanded(child: titleRow)]);
                }

                if (constraints.maxWidth < 460) {
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      titleRow,
                      const SizedBox(height: AppTokens.space8),
                      trailing!,
                    ],
                  );
                }

                return Row(
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    Expanded(child: titleRow),
                    const SizedBox(width: AppTokens.space12),
                    trailing!,
                  ],
                );
              },
            ),
            if (subtitle != null) ...[
              const SizedBox(height: AppTokens.space4),
              Text(
                subtitle!,
                style: AppTypography.captionSans(color: colors.mutedInk),
              ),
            ],
            const SizedBox(height: AppTokens.space16),
          ],
          child,
        ],
      ),
    );
  }
}

/// Semantic inline status banner for errors, warnings, success messages, and hints.
class ShelfdInlineBanner extends StatelessWidget {
  final String message;
  final String? title;
  final ShelfdBannerTone tone;
  final VoidCallback? onDismiss;

  const ShelfdInlineBanner({
    super.key,
    required this.message,
    this.title,
    this.tone = ShelfdBannerTone.info,
    this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final Color bg;
    final Color border;
    final Color fg;
    final IconData icon;

    switch (tone) {
      case ShelfdBannerTone.error:
        bg = colors.errorBg;
        border = colors.errorBorder;
        fg = colors.errorText;
        icon = Icons.error_outline_rounded;
      case ShelfdBannerTone.warning:
        bg = colors.warningBg;
        border = colors.warningBorder;
        fg = colors.warningText;
        icon = Icons.warning_amber_rounded;
      case ShelfdBannerTone.success:
        bg = colors.isDark
            ? colors.accentSecondaryContainer
            : AppTokens.completedBadgeBg;
        border = colors.accentSecondary.withValues(alpha: 0.35);
        fg = colors.accentSecondary;
        icon = Icons.check_circle_outline_rounded;
      case ShelfdBannerTone.info:
        bg = colors.containerLow;
        border = colors.border;
        fg = colors.secondaryInk;
        icon = Icons.info_outline_rounded;
    }

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space16,
        vertical: AppTokens.space12,
      ),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        border: Border.all(color: border),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 1),
            child: Icon(icon, size: 18, color: fg),
          ),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                if (title != null) ...[
                  Text(
                    title!,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: fg,
                    ),
                  ),
                  const SizedBox(height: 2),
                ],
                Text(
                  message,
                  style: AppTypography.bodySans(
                    fontSize: 13,
                    color: fg,
                  ),
                ),
              ],
            ),
          ),
          if (onDismiss != null)
            IconButton(
              icon: const Icon(Icons.close_rounded, size: 16),
              color: fg,
              visualDensity: VisualDensity.compact,
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
              tooltip: 'Dismiss',
              onPressed: onDismiss,
            ),
        ],
      ),
    );
  }
}

/// Centered empty or clean-state presentation card.
class ShelfdEmptyState extends StatelessWidget {
  final IconData icon;
  final String title;
  final String message;
  final Widget? action;
  final bool positive;

  const ShelfdEmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.message,
    this.action,
    this.positive = false,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final circleBg = positive
        ? (colors.isDark
            ? colors.accentSecondaryContainer
            : AppTokens.completedBadgeBg)
        : colors.containerLow;
    final iconColor = positive ? colors.accentSecondary : colors.mutedInk;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(
        vertical: AppTokens.space48,
        horizontal: AppTokens.space24,
      ),
      decoration: BoxDecoration(
        color: colors.surface,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(color: colors.subtleBorder),
      ),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 52,
                height: 52,
                decoration: BoxDecoration(
                  color: circleBg,
                  shape: BoxShape.circle,
                ),
                child: Icon(icon, size: 26, color: iconColor),
              ),
              const SizedBox(height: AppTokens.space16),
              Text(
                title,
                textAlign: TextAlign.center,
                style: AppTypography.titleSerif(
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
              const SizedBox(height: AppTokens.space8),
              Text(
                message,
                textAlign: TextAlign.center,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  color: colors.secondaryInk,
                ),
              ),
              if (action != null) ...[
                const SizedBox(height: AppTokens.space20),
                action!,
              ],
            ],
          ),
        ),
      ),
    );
  }
}

/// Standardized secondary outlined button matching [PrimaryButton] visual language.
class ShelfdSecondaryButton extends StatelessWidget {
  final String label;
  final IconData? icon;
  final VoidCallback? onPressed;
  final bool isLoading;
  final bool compact;

  const ShelfdSecondaryButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.icon,
    this.isLoading = false,
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final child = isLoading
        ? SizedBox(
            width: 16,
            height: 16,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              color: colors.accentPrimary,
            ),
          )
        : Text(
            label,
            style: AppTypography.bodySans(
              fontSize: compact ? 12 : 13,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
          );

    final style = OutlinedButton.styleFrom(
      backgroundColor: colors.surface,
      foregroundColor: colors.ink,
      side: BorderSide(color: colors.border),
      padding: EdgeInsets.symmetric(
        horizontal: compact ? AppTokens.space12 : AppTokens.space16,
        vertical: compact ? AppTokens.space8 : AppTokens.space12,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
      ),
    );

    if (icon != null && !isLoading) {
      return OutlinedButton.icon(
        style: style,
        onPressed: onPressed,
        icon: Icon(icon, size: compact ? 15 : 16),
        label: child,
      );
    }

    return OutlinedButton(
      style: style,
      onPressed: isLoading ? null : onPressed,
      child: child,
    );
  }
}

/// Standardized destructive/danger outlined button.
class ShelfdDangerButton extends StatelessWidget {
  final String label;
  final IconData? icon;
  final VoidCallback? onPressed;
  final bool isLoading;
  final bool compact;

  const ShelfdDangerButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.icon,
    this.isLoading = false,
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final child = isLoading
        ? SizedBox(
            width: 16,
            height: 16,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              color: colors.errorText,
            ),
          )
        : Text(
            label,
            style: AppTypography.bodySans(
              fontSize: compact ? 12 : 13,
              fontWeight: FontWeight.w600,
              color: colors.errorText,
            ),
          );

    final style = OutlinedButton.styleFrom(
      foregroundColor: colors.errorText,
      side: BorderSide(color: colors.errorBorder),
      backgroundColor: colors.errorBg.withValues(alpha: 0.35),
      padding: EdgeInsets.symmetric(
        horizontal: compact ? AppTokens.space12 : AppTokens.space16,
        vertical: compact ? AppTokens.space8 : AppTokens.space12,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
      ),
    );

    if (icon != null && !isLoading) {
      return OutlinedButton.icon(
        style: style,
        onPressed: onPressed,
        icon: Icon(icon, size: compact ? 15 : 16),
        label: child,
      );
    }

    return OutlinedButton(
      style: style,
      onPressed: isLoading ? null : onPressed,
      child: child,
    );
  }
}
