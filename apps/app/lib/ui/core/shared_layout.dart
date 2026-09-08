import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../state/providers.dart';
import 'responsive.dart';
import 'tokens.dart';
import 'typography.dart';

class BentoCard extends StatelessWidget {
  final Widget child;
  final VoidCallback? onTap;
  final EdgeInsetsGeometry padding;
  final Color? backgroundColor;
  final double? width;
  final double? height;

  const BentoCard({
    super.key,
    required this.child,
    this.onTap,
    this.padding = const EdgeInsets.all(AppTokens.space16),
    this.backgroundColor,
    this.width,
    this.height,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cardWidget = Card(
      elevation: 0,
      margin: EdgeInsets.zero,
      color: backgroundColor ?? theme.cardColor,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        side: BorderSide(color: theme.dividerColor),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          width: width,
          height: height,
          padding: padding,
          child: child,
        ),
      ),
    );

    return cardWidget;
  }
}

class FilterPillItem {
  final String id;
  final String label;

  const FilterPillItem({required this.id, required this.label});
}

class FilterPillsRow extends StatelessWidget {
  final List<FilterPillItem> items;
  final String selectedId;
  final ValueChanged<String> onSelected;

  const FilterPillsRow({
    super.key,
    required this.items,
    required this.selectedId,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: items.map((item) {
          final isSelected = item.id == selectedId;
          return Padding(
            padding: const EdgeInsets.only(right: AppTokens.space8),
            child: ChoiceChip(
              label: Text(
                item.label,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                  color: isSelected ? Colors.white : AppTokens.mutedCopy,
                ),
              ),
              selected: isSelected,
              onSelected: (_) => onSelected(item.id),
              selectedColor: AppTokens.charcoalInk,
              backgroundColor: AppTokens.boneSurface,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                side: BorderSide(
                  color: isSelected ? AppTokens.charcoalInk : AppTokens.crispBorder,
                ),
              ),
              showCheckmark: false,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            ),
          );
        }).toList(),
      ),
    );
  }
}

class StatusBadge extends StatelessWidget {
  final String label;
  final Color backgroundColor;
  final Color textColor;

  const StatusBadge({
    super.key,
    required this.label,
    this.backgroundColor = AppTokens.matchBadgeBg,
    this.textColor = AppTokens.matchBadgeText,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppTokens.space8, vertical: AppTokens.space4),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: textColor,
          letterSpacing: 0.2,
        ),
      ),
    );
  }
}

class PrimaryButton extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;
  final bool isLoading;
  final IconData? icon;
  final Color? backgroundColor;
  final Color? foregroundColor;

  const PrimaryButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.isLoading = false,
    this.icon,
    this.backgroundColor,
    this.foregroundColor,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: AppTokens.minTouchTarget,
      child: ElevatedButton(
        style: ElevatedButton.styleFrom(
          backgroundColor: backgroundColor ?? AppTokens.charcoalInk,
          foregroundColor: foregroundColor ?? Colors.white,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          ),
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space20),
        ),
        onPressed: isLoading ? null : onPressed,
        child: isLoading
            ? const SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (icon != null) ...[
                    Icon(icon, size: 18),
                    const SizedBox(width: AppTokens.space8),
                  ],
                  Text(
                    label,
                    style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                  ),
                ],
              ),
      ),
    );
  }
}

class ShelfdTopBar extends StatelessWidget implements PreferredSizeWidget {
  final String title;
  final String? subtitle;
  final List<Widget>? actions;
  final Widget? leading;

  const ShelfdTopBar({
    super.key,
    required this.title,
    this.subtitle,
    this.actions,
    this.leading,
  });

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight + 8);

  @override
  Widget build(BuildContext context) {
    return AppBar(
      leading: leading,
      title: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(title, style: AppTypography.titleSerif(fontSize: 20)),
          if (subtitle != null) ...[
            const SizedBox(height: 2),
            Row(
              children: [
                Container(
                  width: 6,
                  height: 6,
                  decoration: const BoxDecoration(
                    color: Color(0xFF2B8A3E),
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: 4),
                Text(
                  subtitle!,
                  style: AppTypography.bodySans(fontSize: 11, color: AppTokens.mutedCopy),
                ),
              ],
            ),
          ],
        ],
      ),
      actions: actions,
      bottom: PreferredSize(
        preferredSize: const Size.fromHeight(1),
        child: Container(color: AppTokens.crispBorder, height: 1),
      ),
    );
  }
}

class ShelfdBottomNav extends StatelessWidget {
  final int currentIndex;
  final ValueChanged<int> onTap;

  const ShelfdBottomNav({
    super.key,
    required this.currentIndex,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: const BoxDecoration(
        border: Border(top: BorderSide(color: AppTokens.crispBorder, width: 1)),
      ),
      child: NavigationBar(
        selectedIndex: currentIndex,
        onDestinationSelected: onTap,
        backgroundColor: Theme.of(context).scaffoldBackgroundColor,
        elevation: 0,
        height: 64,
        indicatorColor: AppTokens.boneContainer,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.menu_book_outlined),
            selectedIcon: Icon(Icons.menu_book_rounded, color: AppTokens.charcoalInk),
            label: 'Library',
          ),
          NavigationDestination(
            icon: Icon(Icons.saved_search_outlined),
            selectedIcon: Icon(Icons.saved_search_rounded, color: AppTokens.charcoalInk),
            label: 'Search',
          ),
          NavigationDestination(
            icon: Icon(Icons.settings_outlined),
            selectedIcon: Icon(Icons.settings_rounded, color: AppTokens.charcoalInk),
            label: 'Settings',
          ),
        ],
      ),
    );
  }
}

class ShelfdSideNav extends StatelessWidget {
  final int currentIndex;
  final ValueChanged<int> onTap;
  final VoidCallback? onRescan;
  final bool isRescanning;

  const ShelfdSideNav({
    super.key,
    required this.currentIndex,
    required this.onTap,
    this.onRescan,
    this.isRescanning = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      width: AppTokens.sidebarWidth,
      height: double.infinity,
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        border: const Border(
          right: BorderSide(color: AppTokens.crispBorder, width: 1),
        ),
      ),
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space20,
        vertical: AppTokens.space24,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Brand Logo & Title
          Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: AppTokens.charcoalInk,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                ),
                child: const Icon(
                  Icons.auto_stories_rounded,
                  color: AppTokens.boneSurface,
                  size: 20,
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      'Shelfd',
                      style: AppTypography.titleSerif(
                        fontSize: 22,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    Text(
                      'Your Digital Vellum',
                      style: AppTypography.bodySans(
                        fontSize: 11,
                        color: AppTokens.mutedCopy,
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space32),

          // Navigation Links
          _SideNavItem(
            icon: Icons.menu_book_outlined,
            selectedIcon: Icons.menu_book_rounded,
            label: 'Library',
            isSelected: currentIndex == 0,
            onTap: () => onTap(0),
          ),
          const SizedBox(height: AppTokens.space8),
          _SideNavItem(
            icon: Icons.saved_search_outlined,
            selectedIcon: Icons.saved_search_rounded,
            label: 'Semantic Search',
            isSelected: currentIndex == 1,
            onTap: () => onTap(1),
          ),
          const SizedBox(height: AppTokens.space8),
          _SideNavItem(
            icon: Icons.settings_outlined,
            selectedIcon: Icons.settings_rounded,
            label: 'Settings',
            isSelected: currentIndex == 2,
            onTap: () => onTap(2),
          ),

          const Spacer(),

          // Live AI indexing queue progress card
          const _QueueStatusCard(),
          const SizedBox(height: AppTokens.space12),

          // Rescan Library Button (if callback provided)
          if (onRescan != null) ...[
            SizedBox(
              width: double.infinity,
              height: 40,
              child: OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  backgroundColor: AppTokens.charcoalInk,
                  foregroundColor: Colors.white,
                  side: BorderSide.none,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                  padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
                ),
                onPressed: isRescanning ? null : onRescan,
                icon: isRescanning
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                      )
                    : const Icon(Icons.sync_rounded, size: 18),
                label: Text(
                  isRescanning ? 'Scanning...' : 'Rescan Library',
                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
                ),
              ),
            ),
            const SizedBox(height: AppTokens.space16),
          ],

          // Homelab NAS connection status badge
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
            decoration: BoxDecoration(
              color: AppTokens.boneContainer,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: AppTokens.crispBorder),
            ),
            child: Row(
              children: [
                Container(
                  width: 8,
                  height: 8,
                  decoration: const BoxDecoration(
                    color: Color(0xFF2B8A3E),
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    'Homelab NAS - Connected',
                    style: AppTypography.bodySans(
                      fontSize: 11,
                      fontWeight: FontWeight.w500,
                      color: AppTokens.mutedCopy,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SideNavItem extends StatelessWidget {
  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final bool isSelected;
  final VoidCallback onTap;

  const _SideNavItem({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: isSelected ? AppTokens.boneContainer : Colors.transparent,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          height: AppTokens.minTouchTarget,
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            border: isSelected ? Border.all(color: AppTokens.crispBorder) : null,
          ),
          child: Row(
            children: [
              Icon(
                isSelected ? selectedIcon : icon,
                color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                size: 22,
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Text(
                  label,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (isSelected)
                Container(
                  width: 4,
                  height: 16,
                  decoration: BoxDecoration(
                    color: AppTokens.charcoalInk,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _QueueStatusCard extends ConsumerWidget {
  const _QueueStatusCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final queueState = ref.watch(queueProvider);
    final status = queueState.status;

    if (status == null) {
      if (queueState.isLoading) {
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: AppTokens.boneContainer.withValues(alpha: 0.5),
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            border: Border.all(color: AppTokens.crispBorder.withValues(alpha: 0.6)),
          ),
          child: Row(
            children: [
              const SizedBox(
                width: 12,
                height: 12,
                child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTokens.mutedCopy),
              ),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  'Checking queue...',
                  style: AppTypography.bodySans(
                    fontSize: 11,
                    color: AppTokens.mutedCopy,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        );
      }
      return const SizedBox.shrink();
    }

    final isActive = status.isActive || status.pendingChapters > 0 || status.pendingUploads > 0;

    if (!isActive) {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: AppTokens.boneContainer.withValues(alpha: 0.5),
          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          border: Border.all(color: AppTokens.crispBorder.withValues(alpha: 0.6)),
        ),
        child: Row(
          children: [
            const Icon(Icons.check_circle_outline_rounded, size: 14, color: Color(0xFF2B8A3E)),
            const SizedBox(width: AppTokens.space8),
            Expanded(
              child: Text(
                'AI Catalog Synced (${status.totalChapters} ch)',
                style: AppTypography.bodySans(
                  fontSize: 11,
                  fontWeight: FontWeight.w500,
                  color: AppTokens.mutedCopy,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      );
    }

    final progress = (status.progressPercent / 100.0).clamp(0.0, 1.0);
    final title = (status.currentBook != null && status.currentBook!.isNotEmpty)
        ? status.currentBook!
        : (status.pendingUploads > 0 ? 'Processing uploads...' : 'Generating chapter summaries...');

    final subtext = (status.currentChapter != null && status.currentChapter!.isNotEmpty)
        ? status.currentChapter!
        : '${status.indexedChapters} of ${status.totalChapters} indexed (${status.progressPercent.toStringAsFixed(0)}%)';

    return InkWell(
      onTap: () => ref.read(queueProvider.notifier).refresh(),
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Container(
        padding: const EdgeInsets.all(AppTokens.space12),
        decoration: BoxDecoration(
          color: AppTokens.boneSurface,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          border: Border.all(color: AppTokens.crispBorder),
          boxShadow: const [
            BoxShadow(
              color: Color(0x0A000000),
              blurRadius: 4,
              offset: Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(
                    color: const Color(0xFFFFF4E6),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: const Icon(
                    Icons.auto_awesome_rounded,
                    size: 14,
                    color: Color(0xFFD9480F),
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    'AI Indexing',
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: AppTokens.charcoalInk,
                    ),
                  ),
                ),
                Text(
                  '${status.progressPercent.toStringAsFixed(0)}%',
                  style: AppTypography.bodySans(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: const Color(0xFFD9480F),
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space8),
            ClipRRect(
              borderRadius: BorderRadius.circular(2),
              child: LinearProgressIndicator(
                value: progress,
                minHeight: 4,
                backgroundColor: AppTokens.boneContainer,
                valueColor: const AlwaysStoppedAnimation<Color>(Color(0xFFD9480F)),
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              title,
              style: AppTypography.bodySans(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: AppTokens.charcoalInk,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 2),
            Text(
              subtext,
              style: AppTypography.bodySans(
                fontSize: 10,
                color: AppTokens.mutedCopy,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}

class ShelfdAdaptiveScaffold extends StatelessWidget {
  final int currentIndex;
  final ValueChanged<int> onNavTap;
  final PreferredSizeWidget? appBar;
  final Widget body;
  final VoidCallback? onRescan;
  final bool isRescanning;

  const ShelfdAdaptiveScaffold({
    super.key,
    required this.currentIndex,
    required this.onNavTap,
    this.appBar,
    required this.body,
    this.onRescan,
    this.isRescanning = false,
  });

  @override
  Widget build(BuildContext context) {
    final isDesktop = Responsive.isDesktop(context);

    if (isDesktop) {
      return Scaffold(
        body: Row(
          children: [
            ShelfdSideNav(
              currentIndex: currentIndex,
              onTap: onNavTap,
              onRescan: onRescan,
              isRescanning: isRescanning,
            ),
            Expanded(
              child: body,
            ),
          ],
        ),
      );
    }

    return Scaffold(
      appBar: appBar,
      body: body,
      bottomNavigationBar: ShelfdBottomNav(
        currentIndex: currentIndex,
        onTap: onNavTap,
      ),
    );
  }
}

void showShelfdSettingsModal(
  BuildContext context, {
  required Future<void> Function() onLogout,
}) {
  showModalBottomSheet(
    context: context,
    backgroundColor: AppTokens.boneBackground,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
    ),
    builder: (ctx) => SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(AppTokens.space24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                const Icon(Icons.settings_outlined, size: 24, color: AppTokens.charcoalInk),
                const SizedBox(width: AppTokens.space12),
                Text('Settings', style: AppTypography.titleSerif(fontSize: 20)),
                const Spacer(),
                IconButton(
                  icon: const Icon(Icons.close_rounded),
                  onPressed: () => Navigator.of(ctx).pop(),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space16),
            const Divider(color: AppTokens.crispBorder, height: 1),
            const SizedBox(height: AppTokens.space16),
            Row(
              children: [
                const Icon(Icons.dns_outlined, size: 20, color: AppTokens.mutedCopy),
                const SizedBox(width: AppTokens.space12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Homelab Server',
                        style: AppTypography.bodySans(fontSize: 14, fontWeight: FontWeight.w600),
                      ),
                      Text(
                        'Connected to Shelfd Homelab Daemon',
                        style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                      ),
                    ],
                  ),
                ),
                Container(
                  width: 8,
                  height: 8,
                  decoration: const BoxDecoration(
                    color: Color(0xFF2B8A3E),
                    shape: BoxShape.circle,
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space16),
            Consumer(
              builder: (context, ref, _) {
                final queueState = ref.watch(queueProvider);
                final status = queueState.status;
                if (status == null) return const SizedBox.shrink();

                final progress = (status.progressPercent / 100.0).clamp(0.0, 1.0);

                return Container(
                  padding: const EdgeInsets.all(AppTokens.space12),
                  decoration: BoxDecoration(
                    color: AppTokens.boneSurface,
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    border: Border.all(color: AppTokens.crispBorder),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          const Icon(Icons.auto_awesome_rounded, size: 16, color: Color(0xFFD9480F)),
                          const SizedBox(width: AppTokens.space8),
                          Text(
                            'AI Chapter Summaries & Vectors',
                            style: AppTypography.bodySans(fontSize: 13, fontWeight: FontWeight.w600),
                          ),
                          const Spacer(),
                          Text(
                            '${status.progressPercent.toStringAsFixed(0)}%',
                            style: AppTypography.bodySans(
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                              color: const Color(0xFFD9480F),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: AppTokens.space8),
                      ClipRRect(
                        borderRadius: BorderRadius.circular(2),
                        child: LinearProgressIndicator(
                          value: progress,
                          minHeight: 4,
                          backgroundColor: AppTokens.boneContainer,
                          valueColor: const AlwaysStoppedAnimation<Color>(Color(0xFFD9480F)),
                        ),
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Text(
                        '${status.indexedChapters} of ${status.totalChapters} chapters processed (${status.pendingChapters} pending)',
                        style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                      ),
                      if (status.currentBook != null && status.currentBook!.isNotEmpty) ...[
                        const SizedBox(height: 4),
                        Text(
                          'Now processing: ${status.currentBook}',
                          style: AppTypography.bodySans(
                            fontSize: 11,
                            fontWeight: FontWeight.w500,
                            color: AppTokens.charcoalInk,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ],
                  ),
                );
              },
            ),
            const SizedBox(height: AppTokens.space24),
            OutlinedButton.icon(
              style: OutlinedButton.styleFrom(
                foregroundColor: const Color(0xFFC92A2A),
                side: const BorderSide(color: Color(0xFFFFC9C9)),
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
              icon: const Icon(Icons.logout_rounded, size: 18),
              label: const Text('Disconnect / Sign Out'),
              onPressed: () async {
                Navigator.of(ctx).pop();
                await onLogout();
              },
            ),
          ],
        ),
      ),
    ),
  );
}


